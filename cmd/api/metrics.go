package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-chi/chi/v5"
)

// runMetricsRow is a full row from backtest_run_metrics (DDL 002) including ReplacingMergeTree fields.
// Source: services/backtest-engine/migrations/clickhouse/002_backtest_results.up.sql
type runMetricsRow struct {
	runSummary
	Version   uint64    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

// getRunMetrics returns the deduplicated run row from backtest_run_metrics (same logical data as
// summary, plus version/created_at). There is no separate EAV backtest_metrics table in current DDL.
func getRunMetrics(conn driver.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "run_id")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		var out runMetricsRow
		err := conn.QueryRow(ctx, `
			SELECT run_id, strategy_version_id, symbol,
			       period_from, period_to,
			       pnl_abs, pnl_pct, sharpe_ratio, sortino_ratio,
			       max_drawdown_abs, max_drawdown_pct,
			       trades_total, trades_won, trades_lost,
			       profit_factor, expectancy,
			       regime_breakdown_json,
			       version, created_at
			FROM default.backtest_run_metrics FINAL
			WHERE run_id = ?
			ORDER BY version DESC, period_to DESC
			LIMIT 1
		`, runID).Scan(
			&out.RunID, &out.StrategyVersionID, &out.Symbol,
			&out.PeriodFrom, &out.PeriodTo,
			&out.PnLAbs, &out.PnLPct, &out.SharpeRatio, &out.SortinoRatio,
			&out.MaxDrawdownAbs, &out.MaxDrawdownPct,
			&out.TradesTotal, &out.TradesWon, &out.TradesLost,
			&out.ProfitFactor, &out.Expectancy,
			&out.RegimeBreakdown,
			&out.Version, &out.CreatedAt,
		)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type leaderboardResponse struct {
	Metric  string             `json:"metric"`
	Period  string             `json:"period"`
	Entries []leaderboardEntry `json:"entries"`
}

type leaderboardEntry struct {
	Rank        int     `json:"rank"`
	MetricValue float64 `json:"metric_value"`
	runSummary
}

func getLeaderboard(conn driver.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metric := r.URL.Query().Get("metric")
		if metric == "" {
			metric = "sharpe"
		}
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "all"
		}
		orderBy, err := orderByForLeaderboardMetric(metric)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		periodCond, err := periodWhereClause(period)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		sv := r.URL.Query().Get("strategy_version_id")
		if sv == "" {
			sv = r.URL.Query().Get("feature_set_version_id")
		}
		limit := parseLeaderboardLimit(r.URL.Query().Get("limit"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		q := `
			SELECT run_id, strategy_version_id, symbol,
			       period_from, period_to,
			       pnl_abs, pnl_pct, sharpe_ratio, sortino_ratio,
			       max_drawdown_abs, max_drawdown_pct,
			       trades_total, trades_won, trades_lost,
			       profit_factor, expectancy,
			       regime_breakdown_json
			FROM default.backtest_run_metrics FINAL
			WHERE 1=1` + periodCond
		var args []any
		if sv != "" {
			q += " AND strategy_version_id = ?"
			args = append(args, sv)
		}
		q += " ORDER BY " + orderBy + " LIMIT ?"
		args = append(args, limit)

		rows, err := conn.Query(ctx, q, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		var entries []leaderboardEntry
		rank := 0
		for rows.Next() {
			var e leaderboardEntry
			if err := rows.Scan(
				&e.RunID, &e.StrategyVersionID, &e.Symbol,
				&e.PeriodFrom, &e.PeriodTo,
				&e.PnLAbs, &e.PnLPct, &e.SharpeRatio, &e.SortinoRatio,
				&e.MaxDrawdownAbs, &e.MaxDrawdownPct,
				&e.TradesTotal, &e.TradesWon, &e.TradesLost,
				&e.ProfitFactor, &e.Expectancy,
				&e.RegimeBreakdown,
			); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			rank++
			e.Rank = rank
			e.MetricValue = computeMetricValue(metric, e.runSummary)
			entries = append(entries, e)
		}
		if entries == nil {
			entries = []leaderboardEntry{}
		}
		writeJSON(w, http.StatusOK, leaderboardResponse{
			Metric:  metric,
			Period:  period,
			Entries: entries,
		})
	}
}

func computeMetricValue(metric string, s runSummary) float64 {
	switch metric {
	case "sharpe":
		return float64(s.SharpeRatio)
	case "sortino":
		return float64(s.SortinoRatio)
	case "pnl":
		return s.PnLAbs
	case "maxdd":
		return float64(s.MaxDrawdownPct)
	case "winrate":
		d := s.TradesTotal
		if d == 0 {
			d = 1
		}
		return float64(s.TradesWon) / float64(d)
	case "profit_factor":
		return float64(s.ProfitFactor)
	default:
		return 0
	}
}

func orderByForLeaderboardMetric(metric string) (string, error) {
	switch metric {
	case "sharpe":
		return "sharpe_ratio DESC", nil
	case "sortino":
		return "sortino_ratio DESC", nil
	case "pnl":
		return "pnl_abs DESC", nil
	case "maxdd":
		// lower drawdown is better
		return "max_drawdown_pct ASC", nil
	case "winrate":
		return "(trades_won / greatest(trades_total, toUInt32(1))) DESC", nil
	case "profit_factor":
		return "profit_factor DESC", nil
	default:
		return "", fmt.Errorf("invalid metric: use sharpe, sortino, pnl, maxdd, winrate, profit_factor")
	}
}

// periodWhereClause appends a ClickHouse boolean for period_to, UTC calendar relative to now().
func periodWhereClause(period string) (string, error) {
	switch period {
	case "all":
		return "", nil
	case "month":
		return " AND toYYYYMM(period_to) = toYYYYMM(now())", nil
	case "quarter":
		return " AND toQuarter(period_to) = toQuarter(now()) AND toYear(period_to) = toYear(now())", nil
	case "year":
		return " AND toYear(period_to) = toYear(now())", nil
	default:
		return "", fmt.Errorf("invalid period: use all, month, quarter, year")
	}
}

func parseLeaderboardLimit(raw string) int {
	if raw == "" {
		return 20
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 200 {
		return 20
	}
	return n
}

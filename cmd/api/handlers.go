package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-chi/chi/v5"
)

type runSummary struct {
	RunID             string    `json:"run_id"`
	StrategyVersionID string    `json:"strategy_version_id"`
	Symbol            string    `json:"symbol"`
	PeriodFrom        time.Time `json:"period_from"`
	PeriodTo          time.Time `json:"period_to"`
	PnLAbs            float64   `json:"pnl_abs"`
	PnLPct            float32   `json:"pnl_pct"`
	SharpeRatio       float32   `json:"sharpe_ratio"`
	SortinoRatio      float32   `json:"sortino_ratio"`
	MaxDrawdownAbs    float64   `json:"max_drawdown_abs"`
	MaxDrawdownPct    float32   `json:"max_drawdown_pct"`
	TradesTotal       uint32    `json:"trades_total"`
	TradesWon         uint32    `json:"trades_won"`
	TradesLost        uint32    `json:"trades_lost"`
	ProfitFactor      float32   `json:"profit_factor"`
	Expectancy        float64   `json:"expectancy"`
	RegimeBreakdown   string    `json:"regime_breakdown_json"`
}

type tradeRow struct {
	RunID       string    `json:"run_id"`
	TradeIndex  uint32    `json:"trade_index"`
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"`
	EntryTime   time.Time `json:"entry_time"`
	ExitTime    time.Time `json:"exit_time"`
	EntryPrice  float64   `json:"entry_price"`
	ExitPrice   float64   `json:"exit_price"`
	Quantity    float64   `json:"quantity"`
	PnLAbs      float64   `json:"pnl_abs"`
	PnLBps      int32     `json:"pnl_bps"`
	FeesAbs     float64   `json:"fees_abs"`
	SlippageBps int32     `json:"slippage_bps"`
	RegimeCode  string    `json:"regime_code"`
}

type equityRow struct {
	RunID       string    `json:"run_id"`
	TS          time.Time `json:"ts"`
	Equity      float64   `json:"equity"`
	DrawdownAbs float64   `json:"drawdown_abs"`
	DrawdownPct float32   `json:"drawdown_pct"`
}

func getRunSummary(conn driver.Conn, sc *summaryCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "run_id")
		out, err := fetchRunSummaryWithCache(r.Context(), conn, sc, runID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getRunTrades(conn driver.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "run_id")
		limit := parseIntDefault(r.URL.Query().Get("limit"), 200)
		offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, err := conn.Query(ctx, `
			SELECT run_id, trade_index, symbol, side,
			       entry_time, exit_time, entry_price, exit_price,
			       quantity, pnl_abs, pnl_bps, fees_abs,
			       slippage_bps, regime_code
			FROM default.backtest_trades
			WHERE run_id = ?
			ORDER BY trade_index ASC
			LIMIT ? OFFSET ?
		`, runID, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		out := make([]tradeRow, 0)
		for rows.Next() {
			var item tradeRow
			if err := rows.Scan(
				&item.RunID, &item.TradeIndex, &item.Symbol, &item.Side,
				&item.EntryTime, &item.ExitTime, &item.EntryPrice, &item.ExitPrice,
				&item.Quantity, &item.PnLAbs, &item.PnLBps, &item.FeesAbs,
				&item.SlippageBps, &item.RegimeCode,
			); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			out = append(out, item)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getRunEquity(conn driver.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "run_id")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, err := conn.Query(ctx, `
			SELECT run_id, ts, equity, drawdown_abs, drawdown_pct
			FROM default.backtest_equity_curve
			WHERE run_id = ?
			ORDER BY ts ASC
		`, runID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		out := make([]equityRow, 0)
		for rows.Next() {
			var item equityRow
			if err := rows.Scan(&item.RunID, &item.TS, &item.Equity, &item.DrawdownAbs, &item.DrawdownPct); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			out = append(out, item)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func compareRuns(conn driver.Conn, sc *summaryCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		left := chi.URLParam(r, "left_run_id")
		if left == "" {
			left = r.URL.Query().Get("left_run_id")
		}
		right := chi.URLParam(r, "right_run_id")
		if right == "" {
			right = r.URL.Query().Get("right_run_id")
		}
		leftSummary, leftErr := fetchRunSummaryWithCache(r.Context(), conn, sc, left)
		rightSummary, rightErr := fetchRunSummaryWithCache(r.Context(), conn, sc, right)
		if leftErr != nil || rightErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "both run ids must exist"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"left":  leftSummary,
			"right": rightSummary,
			"delta": map[string]any{
				"pnl_abs":          leftSummary.PnLAbs - rightSummary.PnLAbs,
				"pnl_pct":          leftSummary.PnLPct - rightSummary.PnLPct,
				"sharpe_ratio":     leftSummary.SharpeRatio - rightSummary.SharpeRatio,
				"max_drawdown_pct": leftSummary.MaxDrawdownPct - rightSummary.MaxDrawdownPct,
				"trades_total":     int64(leftSummary.TradesTotal) - int64(rightSummary.TradesTotal),
				"profit_factor":    leftSummary.ProfitFactor - rightSummary.ProfitFactor,
				"expectancy":       leftSummary.Expectancy - rightSummary.Expectancy,
			},
		})
	}
}

func compareVersions(conn driver.Conn, sc *summaryCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		leftVersion := r.URL.Query().Get("left_version_id")
		rightVersion := r.URL.Query().Get("right_version_id")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		left, err := latestSummaryForVersionWithCache(ctx, conn, sc, leftVersion)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		right, err := latestSummaryForVersionWithCache(ctx, conn, sc, rightVersion)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"left":  left,
			"right": right,
			"delta": map[string]any{
				"pnl_abs":          left.PnLAbs - right.PnLAbs,
				"pnl_pct":          left.PnLPct - right.PnLPct,
				"sharpe_ratio":     left.SharpeRatio - right.SharpeRatio,
				"max_drawdown_pct": left.MaxDrawdownPct - right.MaxDrawdownPct,
				"trades_total":     int64(left.TradesTotal) - int64(right.TradesTotal),
				"profit_factor":    left.ProfitFactor - right.ProfitFactor,
				"expectancy":       left.Expectancy - right.Expectancy,
			},
		})
	}
}

func fetchRunSummary(ctx context.Context, conn driver.Conn, runID string) (runSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var out runSummary
	err := conn.QueryRow(ctx, `
		SELECT run_id, strategy_version_id, symbol,
		       period_from, period_to,
		       pnl_abs, pnl_pct, sharpe_ratio, sortino_ratio,
		       max_drawdown_abs, max_drawdown_pct,
		       trades_total, trades_won, trades_lost,
		       profit_factor, expectancy,
		       regime_breakdown_json
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
	)
	return out, err
}

// fetchRunSummaryWithCache reuses the same LRU+TTL as GET .../summary (key = run_id).
func fetchRunSummaryWithCache(ctx context.Context, conn driver.Conn, sc *summaryCache, runID string) (runSummary, error) {
	if sc != nil {
		if s, ok := sc.get(runID); ok {
			return s, nil
		}
	}
	s, err := fetchRunSummary(ctx, conn, runID)
	if err != nil {
		return s, err
	}
	if sc != nil {
		sc.put(runID, s)
	}
	return s, nil
}

func latestSummaryForVersion(ctx context.Context, conn driver.Conn, versionID string) (runSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var out runSummary
	err := conn.QueryRow(ctx, `
		SELECT run_id, strategy_version_id, symbol,
		       period_from, period_to,
		       pnl_abs, pnl_pct, sharpe_ratio, sortino_ratio,
		       max_drawdown_abs, max_drawdown_pct,
		       trades_total, trades_won, trades_lost,
		       profit_factor, expectancy,
		       regime_breakdown_json
		FROM default.backtest_run_metrics FINAL
		WHERE strategy_version_id = ?
		ORDER BY period_to DESC, version DESC
		LIMIT 1
	`, versionID).Scan(
		&out.RunID, &out.StrategyVersionID, &out.Symbol,
		&out.PeriodFrom, &out.PeriodTo,
		&out.PnLAbs, &out.PnLPct, &out.SharpeRatio, &out.SortinoRatio,
		&out.MaxDrawdownAbs, &out.MaxDrawdownPct,
		&out.TradesTotal, &out.TradesWon, &out.TradesLost,
		&out.ProfitFactor, &out.Expectancy,
		&out.RegimeBreakdown,
	)
	return out, err
}

func latestSummaryForVersionWithCache(ctx context.Context, conn driver.Conn, sc *summaryCache, versionID string) (runSummary, error) {
	s, err := latestSummaryForVersion(ctx, conn, versionID)
	if err != nil {
		return s, err
	}
	if sc != nil {
		sc.put(s.RunID, s)
	}
	return s, nil
}

func parseIntDefault(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

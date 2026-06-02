(function () {
  const activeCharts = {};

  const PERSON_COLORS = {
    Chris:  { line: "#1d4ed8", fill: "rgba(29,78,216,0.10)",  bar: "#1d4ed8" },
    Ashley: { line: "#60a5fa", fill: "rgba(96,165,250,0.10)", bar: "#60a5fa" },
  };
  function pc(name, key) {
    return (PERSON_COLORS[name] || { line: "#6b7280", fill: "rgba(107,114,128,0.08)", bar: "#6b7280" })[key];
  }

  function destroy(id) {
    if (activeCharts[id]) { activeCharts[id].destroy(); delete activeCharts[id]; }
  }

  const TIME_X = {
    type: "time",
    time: { unit: "month", tooltipFormat: "MMM d, yyyy", displayFormats: { month: "MMM yy" } },
    ticks: { maxRotation: 45 },
  };

  function linearTrend(pts) {
    const n = pts.length;
    if (n < 2) return [];
    const xs = pts.map(p => new Date(p.x).getTime());
    const ys = pts.map(p => p.y);
    const mx = xs.reduce((a, b) => a + b, 0) / n;
    const my = ys.reduce((a, b) => a + b, 0) / n;
    const slope = xs.reduce((s, x, i) => s + (x - mx) * (ys[i] - my), 0) /
                  xs.reduce((s, x) => s + (x - mx) ** 2, 0);
    const intercept = my - slope * mx;
    return [pts[0], pts[n - 1]].map(p => ({
      x: p.x,
      y: Math.max(0, Math.round((slope * new Date(p.x).getTime() + intercept) * 100) / 100),
    }));
  }

  // ── Gross ──────────────────────────────────────────────────────────────────
  function initGrossChart(canvas, data) {
    destroy(canvas.id);
    const multi = data.multi_person;
    const datasets = data.series.map(s => ({
      label: multi ? s.person + " Gross" : "Gross",
      data: s.points.map(p => ({ x: p.x, y: p.y })),
      borderColor: pc(s.person, "line"),
      backgroundColor: pc(s.person, "fill"),
      fill: !multi,
      tension: 0.3,
      pointRadius: 2,
    }));
    // Inflation line — only when viewing a single person (Chris)
    if (!multi && data.series.length === 1) {
      const infPts = data.series[0].points
        .filter(p => p.inflation != null)
        .map(p => ({ x: p.x, y: p.inflation }));
      if (infPts.length) {
        datasets.push({
          label: "SWE Initial Pay Inflation Adjusted",
          data: infPts,
          borderColor: "#f59e0b",
          backgroundColor: "transparent",
          fill: false,
          tension: 0.3,
          borderDash: [6, 3],
          pointRadius: 0,
        });
      }
    }
    // Build inflation lookup for tooltip
    const infMap = {};
    (data.series[0]?.points || []).forEach(p => { if (p.inflation != null) infMap[p.x] = p.inflation; });

    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: { datasets },
      options: {
        responsive: true,
        plugins: {
          legend: { display: datasets.length > 1 },
          tooltip: {
            callbacks: {
              afterBody(items) {
                if (multi) return [];
                const g = items.find(i => i.dataset.label === "Gross");
                if (!g) return [];
                const inf = infMap[g.raw.x];
                if (inf == null) return [];
                const pct = ((g.raw.y - inf) / inf * 100).toFixed(1);
                return [`${pct >= 0 ? "+" : ""}${pct}% vs inflation-adjusted baseline`];
              },
            },
          },
        },
        scales: {
          x: TIME_X,
          y: { ticks: { callback: v => "$" + v.toLocaleString() } },
        },
        elements: { point: { radius: 2 } },
      },
    });
  }

  // ── Tax % ──────────────────────────────────────────────────────────────────
  function initTaxesPctChart(canvas, data) {
    destroy(canvas.id);
    const multi = data.multi_person;
    const taxColors = { Chris: ["#ef4444", "rgba(239,68,68,0.1)"], Ashley: ["#fca5a5", "rgba(252,165,165,0.1)"] };
    const datasets = data.series.map(s => {
      const [line, fill] = taxColors[s.person] || ["#ef4444", "rgba(239,68,68,0.1)"];
      return {
        label: multi ? s.person + " Tax %" : "Tax %",
        data: s.points,
        borderColor: line,
        backgroundColor: fill,
        fill: !multi,
        tension: 0.3,
        pointRadius: 2,
      };
    });
    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: { datasets },
      options: {
        responsive: true,
        plugins: { legend: { display: multi } },
        scales: {
          x: TIME_X,
          y: { ticks: { callback: v => v.toFixed(1) + "%" } },
        },
        elements: { point: { radius: 2 } },
      },
    });
  }

  // ── Savings % ─────────────────────────────────────────────────────────────
  function initSavingsPctChart(canvas, data) {
    destroy(canvas.id);
    const multi = data.multi_person;
    const datasets = data.series.map(s => ({
      label: multi ? s.person + " Savings %" : "Savings %",
      data: s.points,
      borderColor: pc(s.person, "line"),
      backgroundColor: pc(s.person, "fill"),
      fill: !multi,
      tension: 0.3,
      pointRadius: 2,
    }));
    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: { datasets },
      options: {
        responsive: true,
        plugins: { legend: { display: multi } },
        scales: {
          x: TIME_X,
          y: { ticks: { callback: v => v.toFixed(1) + "%" } },
        },
        elements: { point: { radius: 2 } },
      },
    });
  }

  // ── 401k ──────────────────────────────────────────────────────────────────
  function initK401Chart(canvas, data) {
    destroy(canvas.id);
    const multi = data.multi_person;
    const datasets = data.series.map(s => ({
      label: multi ? s.person + " 401k" : "401k",
      data: s.points.map(p => ({ x: p.x, y: p.y })),
      backgroundColor: pc(s.person, "bar"),
      ...(multi ? { stack: "combined" } : {}),
    }));
    // Trend line for single-person view
    if (!multi && data.series.length === 1) {
      const pts = data.series[0].points.map(p => ({ x: p.x, y: p.y }));
      const trendPts = linearTrend(pts);
      if (trendPts.length) {
        datasets.push({
          label: "Trend",
          data: trendPts,
          type: "line",
          borderColor: "#f43f5e",
          backgroundColor: "transparent",
          borderWidth: 2,
          borderDash: [6, 3],
          pointRadius: 0,
          order: 0,
        });
      }
    }
    // Build gross lookup for tooltip
    const grossMap = {};
    (data.series[0]?.points || []).forEach(p => { if (p.gross != null) grossMap[p.x] = p.gross; });

    activeCharts[canvas.id] = new Chart(canvas, {
      type: "bar",
      data: { datasets },
      options: {
        responsive: true,
        plugins: {
          legend: { display: datasets.length > 1 },
          tooltip: {
            callbacks: {
              afterBody(items) {
                if (multi) return [];
                const bar = items.find(i => i.dataset.label === "401k");
                if (!bar) return [];
                const gross = grossMap[bar.raw.x];
                const lines = [];
                if (gross) lines.push(`${(bar.raw.y / gross * 100).toFixed(1)}% of gross`);
                return lines;
              },
            },
          },
        },
        scales: {
          x: { ...TIME_X, stacked: multi },
          y: { stacked: multi, ticks: { callback: v => "$" + v.toLocaleString() } },
        },
      },
    });
  }

  // ── Hours ─────────────────────────────────────────────────────────────────
  function initHoursChart(canvas, data) {
    destroy(canvas.id);
    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: {
        datasets: [{
          label: "Hours",
          data: data.points,
          borderColor: "#f59e0b",
          backgroundColor: "rgba(245,158,11,0.1)",
          fill: true,
          tension: 0.3,
          pointRadius: 2,
        }],
      },
      options: {
        responsive: true,
        plugins: { legend: { display: false } },
        scales: { x: TIME_X, y: { ticks: { callback: v => v + "h" } } },
        elements: { point: { radius: 2 } },
      },
    });
  }

  // ── Annual ─────────────────────────────────────────────────────────────────
  function initAnnualChart(canvas, data) {
    destroy(canvas.id);
    const { years, multi_person: multi, by_person: byPerson } = data;

    let datasets;
    if (!multi) {
      datasets = [
        { label: "Gross",      data: data.gross,     backgroundColor: "#3b82f6" },
        { label: "Take Home",  data: data.takehome,  backgroundColor: "#10b981" },
        { label: "Taxes",      data: data.taxes,     backgroundColor: "#ef4444" },
        { label: "401k",       data: data.k401,      backgroundColor: "#6366f1" },
      ];
    } else {
      const catColors = {
        Chris:  { gross: "#1d4ed8", takehome: "#059669", taxes: "#ef4444", k401: "#4f46e5" },
        Ashley: { gross: "#60a5fa", takehome: "#6ee7b7", taxes: "#fca5a5", k401: "#a5b4fc" },
      };
      const fallback = { gross: "#6b7280", takehome: "#9ca3af", taxes: "#f87171", k401: "#818cf8" };
      datasets = [];
      [["gross","Gross"],["takehome","Take Home"],["taxes","Taxes"],["k401","401k"]].forEach(([cat, catLabel]) => {
        byPerson.forEach(p => {
          const colors = catColors[p.person] || fallback;
          datasets.push({
            label: p.person + " " + catLabel,
            data: p[cat],
            backgroundColor: colors[cat],
            stack: cat,
          });
        });
      });
    }

    activeCharts[canvas.id] = new Chart(canvas, {
      type: "bar",
      data: { labels: years, datasets },
      options: {
        responsive: true,
        plugins: { legend: { display: true } },
        scales: {
          x: { stacked: multi },
          y: { stacked: multi, ticks: { callback: v => "$" + v.toLocaleString() } },
        },
      },
    });
  }

  // ── Retirement ─────────────────────────────────────────────────────────────
  function initRetirementChart(canvas, data) {
    destroy(canvas.id);
    const proj = data.projections || {};
    const toPoints = arr => (arr || []).map(p => ({ x: p.date, y: p.value }));
    const datasets = [];

    if ((data.total_history || []).length)
      datasets.push({ label: "Actual", data: toPoints(data.total_history), borderColor: "#1e293b", backgroundColor: "rgba(30,41,59,0.07)", borderWidth: 3, fill: true, tension: 0.3, pointRadius: 0, order: 0 });
    if (proj.current_rate)
      datasets.push({ label: `Contributing (+$${Math.round(data.monthly_contribution || 0).toLocaleString()}/mo)`, data: toPoints(proj.current_rate), borderColor: "#16a34a", backgroundColor: "transparent", borderDash: [8, 4], borderWidth: 2, pointRadius: 0, order: 1 });
    if (proj.contrib_401k_only)
      datasets.push({ label: `401k Only (+$${Math.round(data.monthly_401k || 0).toLocaleString()}/mo)`, data: toPoints(proj.contrib_401k_only), borderColor: "#059669", backgroundColor: "transparent", borderDash: [5, 5], borderWidth: 1.5, pointRadius: 0, order: 2 });
    if (proj.coast)
      datasets.push({ label: "Coast FIRE (no contributions)", data: toPoints(proj.coast), borderColor: "#2563eb", backgroundColor: "transparent", borderDash: [4, 4], borderWidth: 2, pointRadius: 0, order: 3 });

    const birthYear = data.birth_year || 1990;
    const todayStr = new Date().toISOString().slice(0, 10);
    const cx = proj.contrib_crossed, coastX = proj.coast_crossed, c4kX = proj.contrib_401k_only_crossed;
    const verticals = [];
    if (cx)    verticals.push({ date: cx.date,    color: "#16a34a", label: cx.date.slice(0,4)    + " (age " + cx.age    + ")" });
    if (c4kX)  verticals.push({ date: c4kX.date,  color: "#059669", label: c4kX.date.slice(0,4)  + " 401k (age " + c4kX.age  + ")" });
    if (coastX) verticals.push({ date: coastX.date, color: "#2563eb", label: coastX.date.slice(0,4) + " coast (age " + coastX.age + ")" });

    const target = data.target || 3_500_000;
    const overlayPlugin = {
      id: "overlay",
      afterDatasetsDraw(chart) {
        const { ctx, scales } = chart;
        // flat $3.5M target line
        const ty = scales.y.getPixelForValue(target);
        if (ty >= scales.y.top && ty <= scales.y.bottom) {
          ctx.save();
          ctx.beginPath(); ctx.moveTo(scales.x.left, ty); ctx.lineTo(scales.x.right, ty);
          ctx.strokeStyle = "#d97706"; ctx.lineWidth = 1.5; ctx.setLineDash([6, 4]); ctx.stroke();
          ctx.fillStyle = "#92400e"; ctx.font = "bold 11px sans-serif";
          ctx.fillText("$3.5M target", scales.x.right - 90, ty - 5);
          ctx.restore();
        }
        // today vertical
        const tx = scales.x.getPixelForValue(todayStr);
        if (tx >= scales.x.left && tx <= scales.x.right) {
          ctx.save();
          ctx.beginPath(); ctx.moveTo(tx, scales.y.top); ctx.lineTo(tx, scales.y.bottom);
          ctx.strokeStyle = "rgba(100,100,100,0.35)"; ctx.lineWidth = 1; ctx.setLineDash([3, 3]); ctx.stroke();
          ctx.fillStyle = "#6b7280"; ctx.font = "10px sans-serif";
          ctx.fillText("today", tx + 3, scales.y.top + 12);
          ctx.restore();
        }
        verticals.forEach(v => {
          const x = scales.x.getPixelForValue(v.date);
          if (x < scales.x.left || x > scales.x.right) return;
          ctx.save();
          ctx.beginPath(); ctx.moveTo(x, scales.y.top); ctx.lineTo(x, scales.y.bottom);
          ctx.strokeStyle = v.color; ctx.lineWidth = 1.5; ctx.setLineDash([4, 3]); ctx.stroke();
          ctx.fillStyle = v.color; ctx.font = "bold 10px sans-serif";
          ctx.fillText(v.label, x + 4, scales.y.top + 26);
          ctx.restore();
        });
      },
    };

    function fmt$(v) { return v >= 1e6 ? "$" + (v/1e6).toFixed(2) + "M" : "$" + Math.round(v).toLocaleString(); }

    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: { datasets },
      plugins: [overlayPlugin],
      options: {
        responsive: true,
        plugins: {
          legend: { display: true, position: "top" },
          tooltip: {
            mode: "x", intersect: false,
            callbacks: {
              title: items => {
                if (!items.length) return "";
                const d = new Date(items[0].parsed.x);
                return d.toLocaleDateString("en-US", { month: "short", year: "numeric" }) + " · age " + (d.getFullYear() - birthYear);
              },
              label: i => i.parsed.y != null ? i.dataset.label + ": " + fmt$(i.parsed.y) : null,
            },
          },
        },
        scales: {
          x: { type: "time", min: data.chart_start || "2022-04-01", max: "2069-12-01", time: { unit: "year", displayFormats: { year: "yyyy" } }, ticks: { maxRotation: 0 } },
          y: { min: 0, ticks: { callback: v => v >= 1e6 ? "$" + (v/1e6).toFixed(1) + "M" : "$" + (v/1e3).toFixed(0) + "k" } },
        },
        elements: { point: { radius: 0 } },
      },
    });
  }

  // ── FIRE year ─────────────────────────────────────────────────────────────
  function initFireYearChart(canvas, data) {
    destroy(canvas.id);
    const points = data.points || [];
    const birthYear = data.birth_year;
    const firePlugin = {
      id: "fireTarget",
      afterDatasetsDraw(chart) {
        const { ctx, scales } = chart;
        const ty = scales.y.getPixelForValue(0);
        if (ty < scales.y.top || ty > scales.y.bottom) return;
        ctx.save();
        ctx.beginPath(); ctx.moveTo(scales.x.left, ty); ctx.lineTo(scales.x.right, ty);
        ctx.strokeStyle = "#16a34a"; ctx.lineWidth = 1.5; ctx.setLineDash([6, 4]); ctx.stroke();
        ctx.fillStyle = "#15803d"; ctx.font = "bold 10px sans-serif"; ctx.fillText("FIRE", scales.x.right - 30, ty - 5);
        ctx.restore();
      },
    };
    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: {
        datasets: [
          { label: "401k + Roth", data: points.map(p => ({ x: p.date, y: p.fire_year - parseInt(p.date.slice(0, 4)) })), borderColor: "#059669", fill: false, tension: 0.3, pointRadius: 0, borderDash: [5, 5] },
          { label: "401k only",   data: points.map(p => ({ x: p.date, y: p.fire_year_401k_only - parseInt(p.date.slice(0, 4)) })), borderColor: "#2563eb", backgroundColor: "rgba(37,99,235,0.07)", fill: true, tension: 0.3, pointRadius: 0 },
        ],
      },
      plugins: [firePlugin],
      options: {
        responsive: true,
        plugins: {
          legend: { display: true },
          tooltip: { callbacks: { label: i => { const yrs = Math.round(i.parsed.y); const yr = parseInt(new Date(i.parsed.x).getFullYear()) + yrs; return `${i.dataset.label}: ${yrs} yrs → FIRE ${yr} (age ${yr - birthYear})`; } } },
        },
        scales: { x: { type: "time", time: { unit: "month", displayFormats: { month: "MMM yy" } } }, y: { min: 0, ticks: { callback: v => v + " yrs" } } },
      },
    });
  }

  // ── SWR ───────────────────────────────────────────────────────────────────
  function initSWRChart(canvas, data) {
    destroy(canvas.id);
    const points = data.points || [];
    const targetSWR = data.target_swr;
    const swrPlugin = {
      id: "swrTarget",
      afterDatasetsDraw(chart) {
        const { ctx, scales } = chart;
        const ty = scales.y.getPixelForValue(targetSWR);
        if (ty < scales.y.top || ty > scales.y.bottom) return;
        ctx.save();
        ctx.beginPath(); ctx.moveTo(scales.x.left, ty); ctx.lineTo(scales.x.right, ty);
        ctx.strokeStyle = "#d97706"; ctx.lineWidth = 1.5; ctx.setLineDash([6, 4]); ctx.stroke();
        ctx.fillStyle = "#92400e"; ctx.font = "bold 10px sans-serif"; ctx.fillText("$3.5M target", scales.x.right - 80, ty - 5);
        ctx.restore();
      },
    };
    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: { datasets: [{ label: "Annual SWR at 60", data: points.map(p => ({ x: p.date, y: p.coast_swr_annual })), borderColor: "#10b981", backgroundColor: "rgba(16,185,129,0.07)", fill: true, tension: 0.3, pointRadius: 0 }] },
      plugins: [swrPlugin],
      options: {
        responsive: true,
        plugins: { legend: { display: false } },
        scales: { x: { type: "time", time: { unit: "month", displayFormats: { month: "MMM yy" } } }, y: { min: 0, ticks: { callback: v => v >= 1000 ? "$" + (v/1000).toFixed(0) + "k" : "$" + v } } },
      },
    });
  }

  const chartInits = {
    "chart-gross":       initGrossChart,
    "chart-taxes-pct":   initTaxesPctChart,
    "chart-savings-pct": initSavingsPctChart,
    "chart-401k":        initK401Chart,
    "chart-hours":       initHoursChart,
    "chart-annual":      initAnnualChart,
    "chart-retirement":  initRetirementChart,
    "chart-fire-year":   initFireYearChart,
    "chart-swr":         initSWRChart,
  };

  function initChartsIn(root) {
    root.querySelectorAll("canvas[id]").forEach(canvas => {
      const fn = chartInits[canvas.id];
      if (fn && canvas.dataset.chartData) {
        try {
          fn(canvas, JSON.parse(canvas.dataset.chartData));
        } catch (e) {
          console.error("chart init failed for", canvas.id, e);
        }
      }
    });
  }

  document.addEventListener("htmx:afterSwap", e => initChartsIn(e.detail.target));
  document.addEventListener("DOMContentLoaded", () => initChartsIn(document));
})();

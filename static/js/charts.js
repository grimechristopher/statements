(function () {
  const activeCharts = {};

  function destroy(id) {
    if (activeCharts[id]) {
      activeCharts[id].destroy();
      delete activeCharts[id];
    }
  }

  function initGrossChart(canvas, data) {
    destroy(canvas.id);
    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: {
        labels: data.dates,
        datasets: [
          {
            label: "Gross",
            data: data.gross,
            borderColor: "#3b82f6",
            backgroundColor: "rgba(59,130,246,0.1)",
            fill: true,
            tension: 0.3,
            pointRadius: 2,
          },
          {
            label: "Inflation-Adjusted",
            data: data.inflation,
            borderColor: "#f59e0b",
            borderDash: [5, 5],
            fill: false,
            tension: 0.3,
            pointRadius: 0,
          },
        ],
      },
      options: {
        responsive: true,
        plugins: { legend: { display: true } },
        scales: { y: { ticks: { callback: (v) => "$" + v.toLocaleString() } } },
      },
    });
  }

  function initAnnualChart(canvas, data) {
    destroy(canvas.id);
    activeCharts[canvas.id] = new Chart(canvas, {
      type: "bar",
      data: {
        labels: data.years,
        datasets: [
          { label: "Gross", data: data.gross, backgroundColor: "#3b82f6" },
          { label: "Taxes", data: data.taxes, backgroundColor: "#ef4444" },
          { label: "401k", data: data.k401, backgroundColor: "#6366f1" },
          { label: "HSA", data: data.hsa, backgroundColor: "#10b981" },
          { label: "Cash Saved", data: data.savings, backgroundColor: "#f59e0b" },
        ],
      },
      options: {
        responsive: true,
        plugins: { legend: { display: true } },
        scales: { y: { ticks: { callback: (v) => "$" + v.toLocaleString() } } },
      },
    });
  }

  function initRetirementChart(canvas, data) {
    destroy(canvas.id);
    const toPoints = (arr) => (arr || []).map((p) => ({ x: p.date, y: p.value }));
    const proj = data.projections || {};
    const datasets = [];

    if ((data.total_history || []).length)
      datasets.push({
        label: "Actual",
        data: toPoints(data.total_history),
        borderColor: "#3b82f6",
        backgroundColor: "rgba(59,130,246,0.1)",
        fill: true,
        tension: 0.3,
        pointRadius: 0,
      });
    if (proj.current_rate)
      datasets.push({
        label: "With Contributions",
        data: toPoints(proj.current_rate),
        borderColor: "#6366f1",
        borderDash: [6, 3],
        fill: false,
        tension: 0.3,
        pointRadius: 0,
      });
    if (proj.coast)
      datasets.push({
        label: "Coast (no contrib)",
        data: toPoints(proj.coast),
        borderColor: "#10b981",
        borderDash: [4, 4],
        fill: false,
        tension: 0.3,
        pointRadius: 0,
      });
    if (proj.contrib_401k_only)
      datasets.push({
        label: "401k Only",
        data: toPoints(proj.contrib_401k_only),
        borderColor: "#f59e0b",
        borderDash: [2, 4],
        fill: false,
        tension: 0.3,
        pointRadius: 0,
      });

    const target = data.target || 3_500_000;
    const targetLinePlugin = {
      id: "targetLine",
      afterDatasetsDraw(chart) {
        const { ctx, scales } = chart;
        const ty = scales.y.getPixelForValue(target);
        if (ty < scales.y.top || ty > scales.y.bottom) return;
        ctx.save();
        ctx.beginPath();
        ctx.moveTo(scales.x.left, ty);
        ctx.lineTo(scales.x.right, ty);
        ctx.strokeStyle = "#d97706";
        ctx.lineWidth = 1.5;
        ctx.setLineDash([6, 4]);
        ctx.stroke();
        ctx.fillStyle = "#92400e";
        ctx.font = "bold 10px sans-serif";
        ctx.fillText("$3.5M target", scales.x.right - 90, ty - 5);
        ctx.restore();
      },
    };

    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: { datasets },
      plugins: [targetLinePlugin],
      options: {
        responsive: true,
        plugins: { legend: { display: true, labels: { boxWidth: 16, font: { size: 11 } } } },
        scales: {
          x: {
            type: "time",
            time: { unit: "year", displayFormats: { year: "yyyy" } },
            min: data.chart_start,
          },
          y: {
            ticks: {
              callback: (v) =>
                v >= 1e6 ? "$" + (v / 1e6).toFixed(1) + "M" : "$" + (v / 1e3).toFixed(0) + "k",
            },
          },
        },
      },
    });
  }

  function initFireYearChart(canvas, data) {
    destroy(canvas.id);
    const points = data.points || [];
    const birthYear = data.birth_year;

    const fireYearPlugin = {
      id: "fireTarget",
      afterDatasetsDraw(chart) {
        const { ctx, scales } = chart;
        const ty = scales.y.getPixelForValue(0);
        if (ty < scales.y.top || ty > scales.y.bottom) return;
        ctx.save();
        ctx.beginPath();
        ctx.moveTo(scales.x.left, ty);
        ctx.lineTo(scales.x.right, ty);
        ctx.strokeStyle = "#16a34a";
        ctx.lineWidth = 1.5;
        ctx.setLineDash([6, 4]);
        ctx.stroke();
        ctx.fillStyle = "#15803d";
        ctx.font = "bold 10px sans-serif";
        ctx.fillText("FIRE", scales.x.right - 30, ty - 5);
        ctx.restore();
      },
    };

    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: {
        datasets: [
          {
            label: "401k + Roth",
            data: points.map((p) => ({
              x: p.date,
              y: p.fire_year - parseInt(p.date.slice(0, 4)),
            })),
            borderColor: "#059669",
            fill: false,
            tension: 0.3,
            pointRadius: 0,
            borderDash: [5, 5],
          },
          {
            label: "401k only",
            data: points.map((p) => ({
              x: p.date,
              y: p.fire_year_401k_only - parseInt(p.date.slice(0, 4)),
            })),
            borderColor: "#2563eb",
            backgroundColor: "rgba(37,99,235,0.07)",
            fill: true,
            tension: 0.3,
            pointRadius: 0,
          },
        ],
      },
      plugins: [fireYearPlugin],
      options: {
        responsive: true,
        plugins: {
          legend: { display: true },
          tooltip: {
            callbacks: {
              label: (i) => {
                const yrs = Math.round(i.parsed.y);
                const yr = parseInt(new Date(i.parsed.x).getFullYear()) + yrs;
                return `${i.dataset.label}: ${yrs} yrs → FIRE ${yr} (age ${yr - birthYear})`;
              },
            },
          },
        },
        scales: {
          x: { type: "time", time: { unit: "month", displayFormats: { month: "MMM yy" } } },
          y: { min: 0, ticks: { callback: (v) => v + " yrs" } },
        },
      },
    });
  }

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
        ctx.beginPath();
        ctx.moveTo(scales.x.left, ty);
        ctx.lineTo(scales.x.right, ty);
        ctx.strokeStyle = "#d97706";
        ctx.lineWidth = 1.5;
        ctx.setLineDash([6, 4]);
        ctx.stroke();
        ctx.fillStyle = "#92400e";
        ctx.font = "bold 10px sans-serif";
        ctx.fillText("$3.5M target", scales.x.right - 80, ty - 5);
        ctx.restore();
      },
    };

    activeCharts[canvas.id] = new Chart(canvas, {
      type: "line",
      data: {
        datasets: [
          {
            label: "Annual SWR at 60",
            data: points.map((p) => ({ x: p.date, y: p.coast_swr_annual })),
            borderColor: "#10b981",
            backgroundColor: "rgba(16,185,129,0.07)",
            fill: true,
            tension: 0.3,
            pointRadius: 0,
          },
        ],
      },
      plugins: [swrPlugin],
      options: {
        responsive: true,
        plugins: { legend: { display: false } },
        scales: {
          x: { type: "time", time: { unit: "month", displayFormats: { month: "MMM yy" } } },
          y: {
            min: 0,
            ticks: {
              callback: (v) =>
                v >= 1000 ? "$" + (v / 1000).toFixed(0) + "k" : "$" + v,
            },
          },
        },
      },
    });
  }

  const chartInits = {
    "chart-gross": initGrossChart,
    "chart-annual": initAnnualChart,
    "chart-retirement": initRetirementChart,
    "chart-fire-year": initFireYearChart,
    "chart-swr": initSWRChart,
  };

  function initChartsIn(root) {
    root.querySelectorAll("canvas[id]").forEach((canvas) => {
      const dataEl = root.querySelector(
        `script[type="application/json"][id="data-${canvas.id}"]`
      );
      const fn = chartInits[canvas.id];
      if (fn && dataEl) {
        try {
          fn(canvas, JSON.parse(dataEl.textContent));
        } catch (e) {
          console.error("chart init failed for", canvas.id, e);
        }
      }
    });
  }

  document.addEventListener("htmx:afterSwap", (e) => {
    initChartsIn(e.detail.target);
  });

  document.addEventListener("DOMContentLoaded", () => {
    initChartsIn(document);
  });
})();

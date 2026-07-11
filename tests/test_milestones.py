import pytest

from app.routes.milestones import _linear_trend


def _pt(date, fire_year):
    return {"date": date, "fire_year": fire_year}


def _pt2(date, fire_year, fire_year_401k_only):
    return {"date": date, "fire_year": fire_year, "fire_year_401k_only": fire_year_401k_only}


def test_linear_trend_uses_specified_year_field():
    points = [
        _pt2("2025-01-01", 2045, 2050),  # fire_year yrs=20, 401k-only yrs=25
        _pt2("2026-01-01", 2043, 2047),  # fire_year yrs=17, 401k-only yrs=21
    ]
    trend = _linear_trend(points, "2024-12-19", year_field="fire_year_401k_only")
    assert trend["start_value"] == 25


def test_linear_trend_returns_none_with_fewer_than_two_points():
    points = [_pt("2025-01-01", 2045)]
    assert _linear_trend(points, "2024-12-19") is None


def test_linear_trend_ignores_points_before_start_date():
    points = [
        _pt("2020-01-01", 2100),  # would skew slope wildly if included
        _pt("2025-01-01", 2045),
        _pt("2026-01-01", 2044),
    ]
    trend = _linear_trend(points, "2024-12-19")
    assert trend["start_date"] == "2025-01-01"


def test_linear_trend_on_schedule_has_slope_of_minus_one():
    # yrs-to-retirement drops by exactly 1 for each year that passes
    points = [
        _pt("2025-01-01", 2045),  # yrs = 20
        _pt("2026-01-01", 2045),  # yrs = 19
        _pt("2027-01-01", 2045),  # yrs = 18
    ]
    trend = _linear_trend(points, "2024-12-19")
    assert trend["slope_per_year"] == pytest.approx(-1.0, abs=0.01)
    assert trend["ahead_of_schedule"] is False


def test_linear_trend_ahead_of_schedule_has_steeper_negative_slope():
    # projected FIRE date is pulling in over time -> slope steeper than -1
    points = [
        _pt("2025-01-01", 2045),  # yrs = 20
        _pt("2026-01-01", 2043),  # yrs = 17
        _pt("2027-01-01", 2041),  # yrs = 14
    ]
    trend = _linear_trend(points, "2024-12-19")
    assert trend["slope_per_year"] == pytest.approx(-3.0, abs=0.01)
    assert trend["ahead_of_schedule"] is True


def test_linear_trend_start_value_anchors_to_actual_first_point():
    # start_value must be the real observed value right after the jump,
    # not the regression line's fitted intercept (which undershoots it)
    points = [
        _pt("2025-01-01", 2050),  # yrs = 25 - the actual jump value
        _pt("2025-06-01", 2046),  # yrs = 21
        _pt("2026-01-01", 2044),  # yrs = 18
    ]
    trend = _linear_trend(points, "2024-12-19")
    assert trend["start_value"] == 25


def test_linear_trend_behind_schedule_has_flatter_slope():
    # projected FIRE date is receding -> slope flatter than -1 (near zero)
    points = [
        _pt("2025-01-01", 2045),  # yrs = 20
        _pt("2026-01-01", 2046),  # yrs = 20
        _pt("2027-01-01", 2047),  # yrs = 20
    ]
    trend = _linear_trend(points, "2024-12-19")
    assert trend["slope_per_year"] == 0.0
    assert trend["ahead_of_schedule"] is False

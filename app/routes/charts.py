from flask import Blueprint, render_template
bp = Blueprint("charts", __name__)

@bp.route("/charts")
def charts():
    return render_template("charts.html")

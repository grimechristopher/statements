from flask import Blueprint, render_template
bp = Blueprint("charts", __name__)

@bp.route("/charts")
def charts():
    return "ok"

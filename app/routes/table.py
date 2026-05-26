from flask import Blueprint, render_template
bp = Blueprint("table", __name__)

@bp.route("/")
def index():
    return render_template("table.html")

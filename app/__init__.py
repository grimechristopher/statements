import os
from flask import Flask
from dotenv import load_dotenv
from .db import init_db

load_dotenv()

def create_app(db_path=None):
    app = Flask(__name__, template_folder="templates")
    app.secret_key = os.environ.get("SECRET_KEY", "dev-key-change-me")
    if db_path:
        app.config["DB_PATH"] = db_path

    with app.app_context():
        init_db()

    from .routes.table import bp as table_bp
    from .routes.charts import bp as charts_bp
    from .routes.balances import bp as balances_bp
    from .routes.milestones import bp as milestones_bp
    app.register_blueprint(table_bp)
    app.register_blueprint(charts_bp)
    app.register_blueprint(balances_bp)
    app.register_blueprint(milestones_bp)

    return app

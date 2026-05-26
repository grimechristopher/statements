from flask import Flask
from .db import init_db

def create_app():
    app = Flask(__name__, template_folder="templates")
    app.secret_key = "dev-key-change-me"

    with app.app_context():
        init_db()

    from .routes.table import bp as table_bp
    from .routes.charts import bp as charts_bp
    app.register_blueprint(table_bp)
    app.register_blueprint(charts_bp)

    return app

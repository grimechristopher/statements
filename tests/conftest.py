import pytest
import os
import tempfile
from app import create_app
from app.db import get_db


@pytest.fixture
def app():
    db_fd, db_path = tempfile.mkstemp(suffix=".db")
    os.close(db_fd)

    test_app = create_app(db_path=db_path)
    test_app.config["TESTING"] = True

    yield test_app

    os.unlink(db_path)


@pytest.fixture
def client(app):
    return app.test_client()


@pytest.fixture
def db(app):
    with app.app_context():
        yield get_db()

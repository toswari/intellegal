# Python AI API

## Run Locally

```bash
pip install -e .[dev]
uvicorn py_ai_api.main:app --host 0.0.0.0 --port 8000 --reload
```

Auth-protected internal endpoints require `X-Internal-Service-Token` and the
`INTERNAL_SERVICE_TOKEN` environment variable.

## Tests

Install dev dependencies and run:

```bash
pip install -e .[dev]
pytest
```

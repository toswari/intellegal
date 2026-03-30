import logging
from contextlib import asynccontextmanager
from functools import lru_cache
from typing import Annotated, Any

from fastapi import Depends, FastAPI
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from .auth import require_internal_service_auth
from .analysis import AnalysisPipeline, AnalysisResult
from .config import Settings, get_settings
from .db import check_connection
from .extraction import ExtractionError, ExtractionPipeline, ExtractionResult
from .indexing import IndexPageInput, IndexingPipeline, IndexingResult
from .logging import configure_logging
from .qdrant import QdrantService

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    configure_logging(settings.log_level)
    if settings.database_startup_check:
        check_connection(settings.database_url)
    qdrant_collection = settings.qdrant_collection_name
    if settings.qdrant_startup_check_enabled:
        qdrant_service = QdrantService(settings)
        qdrant_service.startup_check()
        app.state.qdrant_service = qdrant_service
        qdrant_collection = qdrant_service.collection_name
    logger.info(
        "starting service",
        extra={
            "app_env": settings.app_env,
            "qdrant_collection": qdrant_collection,
            "qdrant_url": settings.qdrant_url,
        },
    )
    yield


app = FastAPI(title="Python AI API", version="0.1.0", lifespan=lifespan)


class ExtractJobRequest(BaseModel):
    job_id: str
    request_id: str | None = None
    document_id: str
    storage_uri: str
    mime_type: str | None = None


class AcceptedJobResponse(BaseModel):
    job_id: str
    status: str
    job_type: str
    result: dict[str, Any] | None = None


class IndexJobRequest(BaseModel):
    job_id: str
    request_id: str | None = None
    document_id: str
    version_checksum: str | None = None
    reindex: bool = False
    extracted_text: str | None = None
    pages: list[IndexPageInput] | None = None
    source_uri: str | None = None


class AnalyzeClauseJobRequest(BaseModel):
    job_id: str
    request_id: str | None = None
    check_id: str
    document_ids: list[str] | None = None
    required_clause_text: str
    context_hint: str | None = None


class AnalyzeCompanyNameJobRequest(BaseModel):
    job_id: str
    request_id: str | None = None
    check_id: str
    document_ids: list[str] | None = None
    old_company_name: str
    new_company_name: str | None = None


@lru_cache
def get_extraction_pipeline() -> ExtractionPipeline:
    return ExtractionPipeline()


def get_qdrant_service(settings: Annotated[Settings, Depends(get_settings)]) -> QdrantService:
    return QdrantService(settings)


def get_indexing_pipeline(
    settings: Annotated[Settings, Depends(get_settings)],
    qdrant: Annotated[QdrantService, Depends(get_qdrant_service)],
) -> IndexingPipeline:
    return IndexingPipeline(
        qdrant=qdrant,
        vector_size=settings.qdrant_vector_size,
        chunk_size=settings.index_chunk_size,
        chunk_overlap=settings.index_chunk_overlap,
    )


def get_analysis_pipeline(
    qdrant: Annotated[QdrantService, Depends(get_qdrant_service)],
) -> AnalysisPipeline:
    return AnalysisPipeline(qdrant=qdrant)


@app.get("/internal/v1/health")
def health(settings: Annotated[Settings, Depends(get_settings)]) -> dict[str, str]:
    return {
        "status": "ok",
        "service": settings.app_name,
        "env": settings.app_env,
        "qdrant_collection": settings.qdrant_collection_name,
    }


@app.get("/internal/v1/bootstrap/auth-check", dependencies=[Depends(require_internal_service_auth)])
def auth_check() -> dict[str, str]:
    return {"status": "ok"}


@app.post(
    "/internal/v1/extract",
    status_code=202,
    response_model=AcceptedJobResponse,
    dependencies=[Depends(require_internal_service_auth)],
)
def start_extract_job(
    payload: ExtractJobRequest,
    pipeline: Annotated[ExtractionPipeline, Depends(get_extraction_pipeline)],
) -> AcceptedJobResponse | JSONResponse:
    try:
        result = pipeline.extract_from_uri(payload.storage_uri, payload.mime_type)
    except ExtractionError as exc:
        return JSONResponse(
            status_code=exc.status_code,
            content={
                "error": {
                    "code": exc.code,
                    "message": str(exc),
                    "retriable": exc.retriable,
                    "details": exc.details,
                }
            },
        )

    return AcceptedJobResponse(
        job_id=payload.job_id,
        status="completed",
        job_type="extract",
        result=result.model_dump(),
    )


@app.post(
    "/internal/v1/index",
    status_code=202,
    response_model=AcceptedJobResponse,
    dependencies=[Depends(require_internal_service_auth)],
)
def start_index_job(
    payload: IndexJobRequest,
    pipeline: Annotated[IndexingPipeline, Depends(get_indexing_pipeline)],
) -> AcceptedJobResponse | JSONResponse:
    try:
        result = pipeline.index_document(
            document_id=payload.document_id,
            checksum=payload.version_checksum or "",
            text=payload.extracted_text,
            pages=payload.pages,
            reindex=payload.reindex,
            source_uri=payload.source_uri,
        )
    except ExtractionError as exc:
        return JSONResponse(
            status_code=exc.status_code,
            content={
                "error": {
                    "code": exc.code,
                    "message": str(exc),
                    "retriable": exc.retriable,
                    "details": exc.details,
                }
            },
        )

    return AcceptedJobResponse(
        job_id=payload.job_id,
        status="completed",
        job_type="index",
        result=result.model_dump(),
    )


@app.post(
    "/internal/v1/analyze/clause",
    status_code=202,
    response_model=AcceptedJobResponse,
    dependencies=[Depends(require_internal_service_auth)],
)
def start_clause_analysis_job(
    payload: AnalyzeClauseJobRequest,
    pipeline: Annotated[AnalysisPipeline, Depends(get_analysis_pipeline)],
) -> AcceptedJobResponse:
    result: AnalysisResult = pipeline.analyze_clause(
        required_clause_text=payload.required_clause_text,
        document_ids=payload.document_ids,
    )
    return AcceptedJobResponse(
        job_id=payload.job_id,
        status="completed",
        job_type="analyze_clause",
        result=result.model_dump(),
    )


@app.post(
    "/internal/v1/analyze/company-name",
    status_code=202,
    response_model=AcceptedJobResponse,
    dependencies=[Depends(require_internal_service_auth)],
)
def start_company_name_analysis_job(
    payload: AnalyzeCompanyNameJobRequest,
    pipeline: Annotated[AnalysisPipeline, Depends(get_analysis_pipeline)],
) -> AcceptedJobResponse:
    result: AnalysisResult = pipeline.analyze_company_name(
        old_company_name=payload.old_company_name,
        new_company_name=payload.new_company_name,
        document_ids=payload.document_ids,
    )
    return AcceptedJobResponse(
        job_id=payload.job_id,
        status="completed",
        job_type="analyze_company_name",
        result=result.model_dump(),
    )

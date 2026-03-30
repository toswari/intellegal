from __future__ import annotations

from dataclasses import dataclass

from py_ai_api.extraction import ExtractionPipeline, OCRText


class _FakePDFExtractor:
    def extract_pages(self, payload: bytes) -> list[str]:
        assert payload == b"pdf-bytes"
        return [
            "  First   page\r\nline 1  \n\nline\t2 ",
            "\n\nSecond\u00a0page ",
        ]


@dataclass
class _FakeOCRExtractor:
    confidence: float = 0.81

    def extract(self, payload: bytes) -> OCRText:
        assert payload == b"jpeg-bytes"
        return OCRText(
            text="  Scanned   text \r\nfrom\timage ",
            confidence=self.confidence,
            diagnostics={"engine": "fake-ocr"},
        )


def test_pdf_extraction_normalizes_text_and_preserves_page_boundaries() -> None:
    pipeline = ExtractionPipeline(pdf_extractor=_FakePDFExtractor(), ocr_extractor=_FakeOCRExtractor())

    result = pipeline.extract_bytes(b"pdf-bytes", "application/pdf")

    assert result.mime_type == "application/pdf"
    assert len(result.pages) == 2
    assert result.pages[0].text == "First page\nline 1\n\nline 2"
    assert result.pages[1].text == "Second page"
    assert result.text == "First page\nline 1\n\nline 2\n\f\nSecond page"
    assert result.diagnostics["page_count"] == 2
    assert result.diagnostics["ocr_used"] is False


def test_jpeg_ocr_extraction_uses_ocr_confidence_and_metadata() -> None:
    pipeline = ExtractionPipeline(pdf_extractor=_FakePDFExtractor(), ocr_extractor=_FakeOCRExtractor())

    result = pipeline.extract_bytes(b"jpeg-bytes", "image/jpeg")

    assert result.mime_type == "image/jpeg"
    assert len(result.pages) == 1
    assert result.pages[0].source == "ocr"
    assert result.pages[0].text == "Scanned text\nfrom image"
    assert result.pages[0].confidence == 0.81
    assert result.confidence == 0.81
    assert result.diagnostics["ocr_used"] is True
    assert result.diagnostics["ocr"]["engine"] == "fake-ocr"

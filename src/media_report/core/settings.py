from __future__ import annotations

import os
import tomllib
from pathlib import Path
from typing import Any

import tomli_w
from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

from media_report.core.redaction import redact_secret


def default_config_path() -> Path:
  xdg_home = os.environ.get("XDG_CONFIG_HOME")
  base = Path(xdg_home) if xdg_home else Path.home() / ".config"
  return base / "media-report" / "config.toml"


class AppSettings(BaseSettings):
  model_config = SettingsConfigDict(extra="ignore", frozen=True)

  llm_provider: str = "ollama"
  llm_model: str = "llama3.1"
  openai_api_key: str | None = None
  openai_base_url: str = "https://api.openai.com/v1"
  ollama_base_url: str = "http://localhost:11434"
  whisper_model: str = "small"
  whisper_device: str = "auto"
  output_format: str = "pdf"
  log_level: str = "INFO"
  config_path: Path = Field(default_factory=default_config_path)

  def to_display_dict(self) -> dict[str, Any]:
    return {
      "config_path": str(self.config_path),
      "llm_provider": self.llm_provider,
      "llm_model": self.llm_model,
      "openai_api_key": redact_secret(self.openai_api_key),
      "openai_base_url": self.openai_base_url,
      "ollama_base_url": self.ollama_base_url,
      "whisper_model": self.whisper_model,
      "whisper_device": self.whisper_device,
      "output_format": self.output_format,
      "log_level": self.log_level,
    }


ENV_FIELD_MAP = {
  "MEDIA_REPORT_LLM_PROVIDER": "llm_provider",
  "MEDIA_REPORT_LLM_MODEL": "llm_model",
  "MEDIA_REPORT_OPENAI_API_KEY": "openai_api_key",
  "MEDIA_REPORT_OPENAI_BASE_URL": "openai_base_url",
  "MEDIA_REPORT_OLLAMA_BASE_URL": "ollama_base_url",
  "MEDIA_REPORT_WHISPER_MODEL": "whisper_model",
  "MEDIA_REPORT_WHISPER_DEVICE": "whisper_device",
  "MEDIA_REPORT_OUTPUT_FORMAT": "output_format",
  "MEDIA_REPORT_LOG_LEVEL": "log_level",
}


def redact_settings(settings: AppSettings) -> dict[str, str]:
  return settings.to_display_dict()


def default_config_data() -> dict[str, Any]:
  return {
    "llm": {
      "provider": "ollama",
      "model": "llama3.1",
      "output_format": "pdf",
    },
    "openai": {
      "base_url": "https://api.openai.com/v1",
      "api_key": "",
    },
    "ollama": {
      "base_url": "http://localhost:11434",
    },
    "transcription": {
      "model": "small",
      "device": "auto",
    },
    "logging": {
      "level": "INFO",
    },
  }


def _flatten_config(data: dict[str, Any]) -> dict[str, Any]:
  flattened: dict[str, Any] = {}
  llm = data.get("llm", {})
  openai = data.get("openai", {})
  ollama = data.get("ollama", {})
  transcription = data.get("transcription", {})
  logging = data.get("logging", {})

  if "provider" in llm:
    flattened["llm_provider"] = llm["provider"]
  if "model" in llm:
    flattened["llm_model"] = llm["model"]
  if "output_format" in llm:
    flattened["output_format"] = llm["output_format"]
  if "api_key" in openai:
    flattened["openai_api_key"] = openai["api_key"] or None
  if "base_url" in openai:
    flattened["openai_base_url"] = openai["base_url"]
  if "base_url" in ollama:
    flattened["ollama_base_url"] = ollama["base_url"]
  if "model" in transcription:
    flattened["whisper_model"] = transcription["model"]
  if "device" in transcription:
    flattened["whisper_device"] = transcription["device"]
  if "level" in logging:
    flattened["log_level"] = logging["level"]

  return flattened


def _load_file_data(path: Path) -> dict[str, Any]:
  if not path.exists():
    return {}
  with path.open("rb") as handle:
    return _flatten_config(tomllib.load(handle))


def _load_env_data() -> dict[str, str]:
  data: dict[str, str] = {}
  for env_name, field_name in ENV_FIELD_MAP.items():
    value = os.environ.get(env_name)
    if value is not None:
      data[field_name] = value
  return data


def load_settings(path: Path | None = None) -> AppSettings:
  config_path = path or default_config_path()
  merged = _load_file_data(config_path)
  merged.update(_load_env_data())
  merged["config_path"] = config_path
  return AppSettings.model_validate(merged)


def write_default_config(path: Path | None = None, force: bool = False) -> Path:
  target = path or default_config_path()
  if target.exists() and not force:
    raise FileExistsError(f"Config file already exists: {target}")

  target.parent.mkdir(parents=True, exist_ok=True)
  with target.open("wb") as handle:
    tomli_w.dump(default_config_data(), handle)
  return target

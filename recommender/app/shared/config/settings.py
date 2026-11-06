import os
from pathlib import Path
from dotenv import load_dotenv

_repo_env = Path(__file__).resolve().parents[4] / ".env"
load_dotenv(dotenv_path=_repo_env if _repo_env.is_file() else None)
# LangChain's implicit tracing captures prompt and tool payloads; tracing stays
# off until the application provides a redacting callback.
os.environ["LANGCHAIN_TRACING_V2"] = "false"

PRODUCT_API = os.getenv("PRODUCT_API")
KAFKA_SERVER = os.getenv("KAFKA_BOOTSTRAP_SERVERS", "kafka:9092")

PRODUCT_EVENTS_TOPIC = os.getenv("PRODUCT_EVENTS_TOPIC", "product_events")
INTERACTION_EVENTS_TOPIC = os.getenv("INTERACTION_EVENTS_TOPIC", "interaction_events")
INVENTORY_EVENTS_TOPIC = os.getenv("INVENTORY_EVENTS_TOPIC", "inventory_events")

DATABASE_URL = os.getenv("DATABASE_URL")

ARTIFACTS_BACKEND = os.getenv("ARTIFACTS_BACKEND", "local")
ARTIFACTS_LOCAL_PATH = os.getenv("ARTIFACTS_LOCAL_PATH", "artifacts")

S3_BUCKET_NAME = os.getenv("S3_BUCKET_NAME")
S3_ENDPOINT_URL = os.getenv("S3_ENDPOINT_URL")
S3_ACCESS_KEY_ID = os.getenv("S3_ACCESS_KEY_ID")
S3_SECRET_ACCESS_KEY = os.getenv("S3_SECRET_ACCESS_KEY")
S3_REGION = os.getenv("S3_REGION", "auto")

GRPC_PORT = int(os.getenv("GRPC_PORT", "8080"))

DEEPSEEK_API_KEY = os.getenv("DEEPSEEK_API_KEY", "")
DEEPSEEK_BASE_URL = os.getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
DEEPSEEK_MODEL = os.getenv("DEEPSEEK_MODEL", "deepseek-flash")
# Never reuse AI_API_KEY here: it may belong to an unrelated provider.
AI_ENDPOINT = DEEPSEEK_BASE_URL
AI_API_KEY = DEEPSEEK_API_KEY
AI_MODEL = DEEPSEEK_MODEL
AI_TIMEOUT_SECONDS = float(os.getenv("AI_TIMEOUT_SECONDS", "30"))
AI_MAX_RETRIES = int(os.getenv("AI_MAX_RETRIES", "2"))
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", "")
GEMINI_EMBEDDING_MODEL = os.getenv("GEMINI_EMBEDDING_MODEL", "gemini-embedding-2")
GEMINI_EMBEDDING_DIMENSION = int(os.getenv("GEMINI_EMBEDDING_DIMENSIONS", os.getenv("GEMINI_EMBEDDING_DIMENSION", "768")))
TAVILY_API_KEY = os.getenv("TAVILY_API_KEY", "")
TYPESAFE_API_KEY = os.getenv("TYPESAFE_API_KEY", "")
TYPESAFE_BASE_URL = os.getenv("TYPESAFE_BASE_URL", "https://api.typesafe.ai")
TYPESAFE_JEV_MODEL = os.getenv("TYPESAFE_JEV_MODEL", "jev-1.14.0")
TYPESAFE_JEV_ENDPOINT = os.getenv("TYPESAFE_JEV_ENDPOINT", "https://api.typesafe.ai/v1/systemone")
_DEFAULT_JEV_MODE = "off" if os.getenv("NODE_ENV", "").strip().lower() == "production" else "shadow"
JEV_MODE = os.getenv("JEV_MODE", _DEFAULT_JEV_MODE).strip().lower()
if JEV_MODE not in {"off", "shadow", "active"}:
    raise ValueError("JEV_MODE must be off, shadow, or active")
JEV_RERANK_ENABLED = os.getenv("JEV_RERANK_ENABLED", "false").strip().lower() == "true"
LANGSMITH_API_KEY = os.getenv("LANGSMITH_API_KEY", "")
LANGSMITH_ENDPOINT = os.getenv("LANGSMITH_ENDPOINT", "https://api.smith.langchain.com")
LANGSMITH_PROJECT = os.getenv("LANGSMITH_PROJECT", "goshopx-recommender")
CHAT_MEMORY_ENABLED = os.getenv("CHAT_MEMORY_ENABLED", "true").strip().lower() == "true"
CHAT_SESSION_SIGNING_KEY = os.getenv("CHAT_SESSION_SIGNING_KEY", os.getenv("JWT_SECRET", ""))

MILVUS_URI = os.getenv("MILVUS_URI", "http://milvus:19530")
MILVUS_TOKEN = os.getenv("MILVUS_TOKEN", "")
MILVUS_COLLECTION = os.getenv("MILVUS_COLLECTION", "product_catalog_gemini_v2" if GEMINI_API_KEY else "product_catalog_hash_v1")
_DEFAULT_VECTOR_DIMENSION = GEMINI_EMBEDDING_DIMENSION if GEMINI_API_KEY else 512
TEXT_EMBEDDING_DIMENSION = int(os.getenv("TEXT_EMBEDDING_DIMENSIONS", os.getenv("TEXT_EMBEDDING_DIMENSION", str(_DEFAULT_VECTOR_DIMENSION))))
IMAGE_EMBEDDING_DIMENSION = int(os.getenv("IMAGE_EMBEDDING_DIMENSIONS", os.getenv("IMAGE_EMBEDDING_DIMENSION", str(_DEFAULT_VECTOR_DIMENSION))))


import os
from dotenv import load_dotenv

load_dotenv()

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
AI_ENDPOINT = os.getenv("DEEPSEEK_BASE_URL", os.getenv("AI_ENDPOINT", "https://api.deepseek.com/chat/completions"))
AI_API_KEY = DEEPSEEK_API_KEY or os.getenv("AI_API_KEY", "")
AI_MODEL = os.getenv("DEEPSEEK_MODEL", os.getenv("AI_MODEL", "deepseek-flash"))
AI_TIMEOUT_SECONDS = float(os.getenv("AI_TIMEOUT_SECONDS", "30"))
AI_MAX_RETRIES = int(os.getenv("AI_MAX_RETRIES", "2"))
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", "")
GEMINI_EMBEDDING_MODEL = os.getenv("GEMINI_EMBEDDING_MODEL", "gemini-embedding-2")
GEMINI_EMBEDDING_DIMENSION = int(os.getenv("GEMINI_EMBEDDING_DIMENSION", "768"))

MILVUS_URI = os.getenv("MILVUS_URI", "http://milvus:19530")
MILVUS_TOKEN = os.getenv("MILVUS_TOKEN", "")
MILVUS_COLLECTION = os.getenv("MILVUS_COLLECTION", "product_catalog_gemini_v2" if GEMINI_API_KEY else "product_catalog_hash_v1")
_DEFAULT_VECTOR_DIMENSION = GEMINI_EMBEDDING_DIMENSION if GEMINI_API_KEY else 512
TEXT_EMBEDDING_DIMENSION = int(os.getenv("TEXT_EMBEDDING_DIMENSION", str(_DEFAULT_VECTOR_DIMENSION)))
IMAGE_EMBEDDING_DIMENSION = int(os.getenv("IMAGE_EMBEDDING_DIMENSION", str(_DEFAULT_VECTOR_DIMENSION)))


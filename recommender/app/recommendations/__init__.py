"""Recommendation package exports without eager-importing database modules."""

__all__ = ["RecommendationService", "load_service", "train_and_save"]


def __getattr__(name: str):
    if name == "RecommendationService":
        from recommendations.service import RecommendationService
        return RecommendationService
    if name == "load_service":
        from recommendations.load import load_service
        return load_service
    if name == "train_and_save":
        from recommendations.train import train_and_save
        return train_and_save
    raise AttributeError(name)

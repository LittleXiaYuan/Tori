from enum import Enum

class PostV1ChatBodyThinkingLevel(str, Enum):
    AUTO = "auto"
    DEEP = "deep"
    NONE = "none"

    def __str__(self) -> str:
        return str(self.value)

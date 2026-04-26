from enum import Enum

class PostV1MemoryAddBodyLayer(str, Enum):
    LONG = "long"
    MID = "mid"
    SHORT = "short"

    def __str__(self) -> str:
        return str(self.value)

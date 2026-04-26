from enum import Enum

class GetHealthzResponse200Status(str, Enum):
    DEGRADED = "degraded"
    OK = "ok"

    def __str__(self) -> str:
        return str(self.value)

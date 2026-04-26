from enum import Enum

class GetHealthzResponse200BreakerState(str, Enum):
    CLOSED = "closed"
    HALF_OPEN = "half-open"
    OPEN = "open"

    def __str__(self) -> str:
        return str(self.value)

from enum import Enum

class PostV1TasksResponse200Status(str, Enum):
    CANCELLED = "cancelled"
    COMPLETED = "completed"
    FAILED = "failed"
    PAUSED = "paused"
    PENDING = "pending"
    RUNNING = "running"

    def __str__(self) -> str:
        return str(self.value)

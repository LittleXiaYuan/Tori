from enum import Enum

class PostV1TasksRunResponse200Status(str, Enum):
    QUEUED = "queued"
    RUNNING = "running"

    def __str__(self) -> str:
        return str(self.value)

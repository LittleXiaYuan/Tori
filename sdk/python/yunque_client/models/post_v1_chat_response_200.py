from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast

if TYPE_CHECKING:
  from ..models.post_v1_chat_response_200_emotion import PostV1ChatResponse200Emotion
  from ..models.post_v1_chat_response_200_usage import PostV1ChatResponse200Usage





T = TypeVar("T", bound="PostV1ChatResponse200")



@_attrs_define
class PostV1ChatResponse200:
    """ 
        Attributes:
            emotion (PostV1ChatResponse200Emotion | Unset):
            id (str | Unset):
            latency_ms (int | Unset):
            reply (str | Unset): Assistant reply.
            session_id (str | Unset):
            task_id (str | Unset):
            trace_id (str | Unset):
            usage (PostV1ChatResponse200Usage | Unset):
     """

    emotion: PostV1ChatResponse200Emotion | Unset = UNSET
    id: str | Unset = UNSET
    latency_ms: int | Unset = UNSET
    reply: str | Unset = UNSET
    session_id: str | Unset = UNSET
    task_id: str | Unset = UNSET
    trace_id: str | Unset = UNSET
    usage: PostV1ChatResponse200Usage | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.post_v1_chat_response_200_emotion import PostV1ChatResponse200Emotion
        from ..models.post_v1_chat_response_200_usage import PostV1ChatResponse200Usage
        emotion: dict[str, Any] | Unset = UNSET
        if not isinstance(self.emotion, Unset):
            emotion = self.emotion.to_dict()

        id = self.id

        latency_ms = self.latency_ms

        reply = self.reply

        session_id = self.session_id

        task_id = self.task_id

        trace_id = self.trace_id

        usage: dict[str, Any] | Unset = UNSET
        if not isinstance(self.usage, Unset):
            usage = self.usage.to_dict()


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
        })
        if emotion is not UNSET:
            field_dict["emotion"] = emotion
        if id is not UNSET:
            field_dict["id"] = id
        if latency_ms is not UNSET:
            field_dict["latency_ms"] = latency_ms
        if reply is not UNSET:
            field_dict["reply"] = reply
        if session_id is not UNSET:
            field_dict["session_id"] = session_id
        if task_id is not UNSET:
            field_dict["task_id"] = task_id
        if trace_id is not UNSET:
            field_dict["trace_id"] = trace_id
        if usage is not UNSET:
            field_dict["usage"] = usage

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.post_v1_chat_response_200_emotion import PostV1ChatResponse200Emotion
        from ..models.post_v1_chat_response_200_usage import PostV1ChatResponse200Usage
        d = dict(src_dict)
        _emotion = d.pop("emotion", UNSET)
        emotion: PostV1ChatResponse200Emotion | Unset
        if isinstance(_emotion,  Unset):
            emotion = UNSET
        else:
            emotion = PostV1ChatResponse200Emotion.from_dict(_emotion)




        id = d.pop("id", UNSET)

        latency_ms = d.pop("latency_ms", UNSET)

        reply = d.pop("reply", UNSET)

        session_id = d.pop("session_id", UNSET)

        task_id = d.pop("task_id", UNSET)

        trace_id = d.pop("trace_id", UNSET)

        _usage = d.pop("usage", UNSET)
        usage: PostV1ChatResponse200Usage | Unset
        if isinstance(_usage,  Unset):
            usage = UNSET
        else:
            usage = PostV1ChatResponse200Usage.from_dict(_usage)




        post_v1_chat_response_200 = cls(
            emotion=emotion,
            id=id,
            latency_ms=latency_ms,
            reply=reply,
            session_id=session_id,
            task_id=task_id,
            trace_id=trace_id,
            usage=usage,
        )


        post_v1_chat_response_200.additional_properties = d
        return post_v1_chat_response_200

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties

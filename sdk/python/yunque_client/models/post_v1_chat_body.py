from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.post_v1_chat_body_thinking_level import PostV1ChatBodyThinkingLevel
from ..types import UNSET, Unset
from typing import cast

if TYPE_CHECKING:
  from ..models.post_v1_chat_body_messages_item import PostV1ChatBodyMessagesItem





T = TypeVar("T", bound="PostV1ChatBody")



@_attrs_define
class PostV1ChatBody:
    """ 
        Attributes:
            messages (list[PostV1ChatBodyMessagesItem]): Chat message history (max 100 entries, each ≤32000 chars).
            class_id (str | Unset):
            platform (str | Unset): Target platform for sticker suggestions (qq/feishu/discord/...).
            session_id (str | Unset): Conversation session id (created automatically if blank).
            student_id (str | Unset):
            task_id (str | Unset):
            teacher_id (str | Unset):
            thinking_level (PostV1ChatBodyThinkingLevel | Unset): Override model tier for thinking budget.
     """

    messages: list[PostV1ChatBodyMessagesItem]
    class_id: str | Unset = UNSET
    platform: str | Unset = UNSET
    session_id: str | Unset = UNSET
    student_id: str | Unset = UNSET
    task_id: str | Unset = UNSET
    teacher_id: str | Unset = UNSET
    thinking_level: PostV1ChatBodyThinkingLevel | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.post_v1_chat_body_messages_item import PostV1ChatBodyMessagesItem
        messages = []
        for messages_item_data in self.messages:
            messages_item = messages_item_data.to_dict()
            messages.append(messages_item)



        class_id = self.class_id

        platform = self.platform

        session_id = self.session_id

        student_id = self.student_id

        task_id = self.task_id

        teacher_id = self.teacher_id

        thinking_level: str | Unset = UNSET
        if not isinstance(self.thinking_level, Unset):
            thinking_level = self.thinking_level.value



        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "messages": messages,
        })
        if class_id is not UNSET:
            field_dict["class_id"] = class_id
        if platform is not UNSET:
            field_dict["platform"] = platform
        if session_id is not UNSET:
            field_dict["session_id"] = session_id
        if student_id is not UNSET:
            field_dict["student_id"] = student_id
        if task_id is not UNSET:
            field_dict["task_id"] = task_id
        if teacher_id is not UNSET:
            field_dict["teacher_id"] = teacher_id
        if thinking_level is not UNSET:
            field_dict["thinking_level"] = thinking_level

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.post_v1_chat_body_messages_item import PostV1ChatBodyMessagesItem
        d = dict(src_dict)
        messages = []
        _messages = d.pop("messages")
        for messages_item_data in (_messages):
            messages_item = PostV1ChatBodyMessagesItem.from_dict(messages_item_data)



            messages.append(messages_item)


        class_id = d.pop("class_id", UNSET)

        platform = d.pop("platform", UNSET)

        session_id = d.pop("session_id", UNSET)

        student_id = d.pop("student_id", UNSET)

        task_id = d.pop("task_id", UNSET)

        teacher_id = d.pop("teacher_id", UNSET)

        _thinking_level = d.pop("thinking_level", UNSET)
        thinking_level: PostV1ChatBodyThinkingLevel | Unset
        if isinstance(_thinking_level,  Unset):
            thinking_level = UNSET
        else:
            thinking_level = PostV1ChatBodyThinkingLevel(_thinking_level)




        post_v1_chat_body = cls(
            messages=messages,
            class_id=class_id,
            platform=platform,
            session_id=session_id,
            student_id=student_id,
            task_id=task_id,
            teacher_id=teacher_id,
            thinking_level=thinking_level,
        )


        post_v1_chat_body.additional_properties = d
        return post_v1_chat_body

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

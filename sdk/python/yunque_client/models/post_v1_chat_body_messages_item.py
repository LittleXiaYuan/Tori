from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.post_v1_chat_body_messages_item_role import PostV1ChatBodyMessagesItemRole
from ..types import UNSET, Unset






T = TypeVar("T", bound="PostV1ChatBodyMessagesItem")



@_attrs_define
class PostV1ChatBodyMessagesItem:
    """ 
        Attributes:
            content (str):
            role (PostV1ChatBodyMessagesItemRole): OpenAI-style role.
            name (str | Unset):
            tool_call_id (str | Unset):
     """

    content: str
    role: PostV1ChatBodyMessagesItemRole
    name: str | Unset = UNSET
    tool_call_id: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        content = self.content

        role = self.role.value

        name = self.name

        tool_call_id = self.tool_call_id


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "content": content,
            "role": role,
        })
        if name is not UNSET:
            field_dict["name"] = name
        if tool_call_id is not UNSET:
            field_dict["tool_call_id"] = tool_call_id

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        content = d.pop("content")

        role = PostV1ChatBodyMessagesItemRole(d.pop("role"))




        name = d.pop("name", UNSET)

        tool_call_id = d.pop("tool_call_id", UNSET)

        post_v1_chat_body_messages_item = cls(
            content=content,
            role=role,
            name=name,
            tool_call_id=tool_call_id,
        )


        post_v1_chat_body_messages_item.additional_properties = d
        return post_v1_chat_body_messages_item

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

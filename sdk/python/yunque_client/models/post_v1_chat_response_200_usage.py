from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset






T = TypeVar("T", bound="PostV1ChatResponse200Usage")



@_attrs_define
class PostV1ChatResponse200Usage:
    """ 
        Attributes:
            completion_tokens (int | Unset):
            prompt_tokens (int | Unset):
            total_tokens (int | Unset):
     """

    completion_tokens: int | Unset = UNSET
    prompt_tokens: int | Unset = UNSET
    total_tokens: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        completion_tokens = self.completion_tokens

        prompt_tokens = self.prompt_tokens

        total_tokens = self.total_tokens


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
        })
        if completion_tokens is not UNSET:
            field_dict["completion_tokens"] = completion_tokens
        if prompt_tokens is not UNSET:
            field_dict["prompt_tokens"] = prompt_tokens
        if total_tokens is not UNSET:
            field_dict["total_tokens"] = total_tokens

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        completion_tokens = d.pop("completion_tokens", UNSET)

        prompt_tokens = d.pop("prompt_tokens", UNSET)

        total_tokens = d.pop("total_tokens", UNSET)

        post_v1_chat_response_200_usage = cls(
            completion_tokens=completion_tokens,
            prompt_tokens=prompt_tokens,
            total_tokens=total_tokens,
        )


        post_v1_chat_response_200_usage.additional_properties = d
        return post_v1_chat_response_200_usage

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

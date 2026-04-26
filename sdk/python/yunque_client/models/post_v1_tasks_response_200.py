from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.post_v1_tasks_response_200_status import PostV1TasksResponse200Status
from ..types import UNSET, Unset
from dateutil.parser import isoparse
from typing import cast
import datetime






T = TypeVar("T", bound="PostV1TasksResponse200")



@_attrs_define
class PostV1TasksResponse200:
    """ Created Task object (full task.Task schema).

        Attributes:
            created_at (datetime.datetime | Unset):
            description (str | Unset):
            id (str | Unset):
            status (PostV1TasksResponse200Status | Unset):
            title (str | Unset):
     """

    created_at: datetime.datetime | Unset = UNSET
    description: str | Unset = UNSET
    id: str | Unset = UNSET
    status: PostV1TasksResponse200Status | Unset = UNSET
    title: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        created_at: str | Unset = UNSET
        if not isinstance(self.created_at, Unset):
            created_at = self.created_at.isoformat()

        description = self.description

        id = self.id

        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value


        title = self.title


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
        })
        if created_at is not UNSET:
            field_dict["created_at"] = created_at
        if description is not UNSET:
            field_dict["description"] = description
        if id is not UNSET:
            field_dict["id"] = id
        if status is not UNSET:
            field_dict["status"] = status
        if title is not UNSET:
            field_dict["title"] = title

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        _created_at = d.pop("created_at", UNSET)
        created_at: datetime.datetime | Unset
        if isinstance(_created_at,  Unset):
            created_at = UNSET
        else:
            created_at = isoparse(_created_at)




        description = d.pop("description", UNSET)

        id = d.pop("id", UNSET)

        _status = d.pop("status", UNSET)
        status: PostV1TasksResponse200Status | Unset
        if isinstance(_status,  Unset):
            status = UNSET
        else:
            status = PostV1TasksResponse200Status(_status)




        title = d.pop("title", UNSET)

        post_v1_tasks_response_200 = cls(
            created_at=created_at,
            description=description,
            id=id,
            status=status,
            title=title,
        )


        post_v1_tasks_response_200.additional_properties = d
        return post_v1_tasks_response_200

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

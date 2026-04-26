from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast

if TYPE_CHECKING:
  from ..models.post_v1_tasks_body_constraints import PostV1TasksBodyConstraints





T = TypeVar("T", bound="PostV1TasksBody")



@_attrs_define
class PostV1TasksBody:
    """ 
        Attributes:
            description (str): Required goal description; the planner uses this to decompose.
            constraints (PostV1TasksBodyConstraints | Unset): TaskConstraints — budget, timeouts, deny-tools, etc.
            title (str | Unset): Optional human-readable title.
     """

    description: str
    constraints: PostV1TasksBodyConstraints | Unset = UNSET
    title: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.post_v1_tasks_body_constraints import PostV1TasksBodyConstraints
        description = self.description

        constraints: dict[str, Any] | Unset = UNSET
        if not isinstance(self.constraints, Unset):
            constraints = self.constraints.to_dict()

        title = self.title


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "description": description,
        })
        if constraints is not UNSET:
            field_dict["constraints"] = constraints
        if title is not UNSET:
            field_dict["title"] = title

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.post_v1_tasks_body_constraints import PostV1TasksBodyConstraints
        d = dict(src_dict)
        description = d.pop("description")

        _constraints = d.pop("constraints", UNSET)
        constraints: PostV1TasksBodyConstraints | Unset
        if isinstance(_constraints,  Unset):
            constraints = UNSET
        else:
            constraints = PostV1TasksBodyConstraints.from_dict(_constraints)




        title = d.pop("title", UNSET)

        post_v1_tasks_body = cls(
            description=description,
            constraints=constraints,
            title=title,
        )


        post_v1_tasks_body.additional_properties = d
        return post_v1_tasks_body

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

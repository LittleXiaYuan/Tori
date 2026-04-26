from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset






T = TypeVar("T", bound="GenerateCogniBody")



@_attrs_define
class GenerateCogniBody:
    """ 
        Attributes:
            description (str): Natural-language description of the desired Cogni (e.g. "a code-review cogni that focuses on
                Go test coverage").
            auto_save (bool | Unset): If true, persist the generated declaration to the cogni directory immediately.
                Default: False.
     """

    description: str
    auto_save: bool | Unset = False
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        description = self.description

        auto_save = self.auto_save


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "description": description,
        })
        if auto_save is not UNSET:
            field_dict["auto_save"] = auto_save

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        description = d.pop("description")

        auto_save = d.pop("auto_save", UNSET)

        generate_cogni_body = cls(
            description=description,
            auto_save=auto_save,
        )


        generate_cogni_body.additional_properties = d
        return generate_cogni_body

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

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast

if TYPE_CHECKING:
  from ..models.generate_cogni_response_200_declaration import GenerateCogniResponse200Declaration





T = TypeVar("T", bound="GenerateCogniResponse200")



@_attrs_define
class GenerateCogniResponse200:
    """ 
        Attributes:
            declaration (GenerateCogniResponse200Declaration | Unset): Generated Cogni declaration (see cogni.Declaration in
                pkg/cogni).
            path (str | Unset): Filesystem path of the saved declaration (when auto_save=true).
            saved (bool | Unset):
     """

    declaration: GenerateCogniResponse200Declaration | Unset = UNSET
    path: str | Unset = UNSET
    saved: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.generate_cogni_response_200_declaration import GenerateCogniResponse200Declaration
        declaration: dict[str, Any] | Unset = UNSET
        if not isinstance(self.declaration, Unset):
            declaration = self.declaration.to_dict()

        path = self.path

        saved = self.saved


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
        })
        if declaration is not UNSET:
            field_dict["declaration"] = declaration
        if path is not UNSET:
            field_dict["path"] = path
        if saved is not UNSET:
            field_dict["saved"] = saved

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.generate_cogni_response_200_declaration import GenerateCogniResponse200Declaration
        d = dict(src_dict)
        _declaration = d.pop("declaration", UNSET)
        declaration: GenerateCogniResponse200Declaration | Unset
        if isinstance(_declaration,  Unset):
            declaration = UNSET
        else:
            declaration = GenerateCogniResponse200Declaration.from_dict(_declaration)




        path = d.pop("path", UNSET)

        saved = d.pop("saved", UNSET)

        generate_cogni_response_200 = cls(
            declaration=declaration,
            path=path,
            saved=saved,
        )


        generate_cogni_response_200.additional_properties = d
        return generate_cogni_response_200

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

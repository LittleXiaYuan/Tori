from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast

if TYPE_CHECKING:
  from ..models.post_v1_memory_search_response_200_results_item import PostV1MemorySearchResponse200ResultsItem





T = TypeVar("T", bound="PostV1MemorySearchResponse200")



@_attrs_define
class PostV1MemorySearchResponse200:
    """ 
        Attributes:
            count (int | Unset):
            results (list[PostV1MemorySearchResponse200ResultsItem] | Unset):
     """

    count: int | Unset = UNSET
    results: list[PostV1MemorySearchResponse200ResultsItem] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.post_v1_memory_search_response_200_results_item import PostV1MemorySearchResponse200ResultsItem
        count = self.count

        results: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.results, Unset):
            results = []
            for results_item_data in self.results:
                results_item = results_item_data.to_dict()
                results.append(results_item)




        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
        })
        if count is not UNSET:
            field_dict["count"] = count
        if results is not UNSET:
            field_dict["results"] = results

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.post_v1_memory_search_response_200_results_item import PostV1MemorySearchResponse200ResultsItem
        d = dict(src_dict)
        count = d.pop("count", UNSET)

        _results = d.pop("results", UNSET)
        results: list[PostV1MemorySearchResponse200ResultsItem] | Unset = UNSET
        if _results is not UNSET:
            results = []
            for results_item_data in _results:
                results_item = PostV1MemorySearchResponse200ResultsItem.from_dict(results_item_data)



                results.append(results_item)


        post_v1_memory_search_response_200 = cls(
            count=count,
            results=results,
        )


        post_v1_memory_search_response_200.additional_properties = d
        return post_v1_memory_search_response_200

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

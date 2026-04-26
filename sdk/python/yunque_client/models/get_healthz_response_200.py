from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.get_healthz_response_200_breaker_state import GetHealthzResponse200BreakerState
from ..models.get_healthz_response_200_status import GetHealthzResponse200Status
from ..types import UNSET, Unset






T = TypeVar("T", bound="GetHealthzResponse200")



@_attrs_define
class GetHealthzResponse200:
    """ 
        Attributes:
            breaker_state (GetHealthzResponse200BreakerState | Unset):
            status (GetHealthzResponse200Status | Unset):
            uptime_sec (int | Unset):
            version (str | Unset):
     """

    breaker_state: GetHealthzResponse200BreakerState | Unset = UNSET
    status: GetHealthzResponse200Status | Unset = UNSET
    uptime_sec: int | Unset = UNSET
    version: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        breaker_state: str | Unset = UNSET
        if not isinstance(self.breaker_state, Unset):
            breaker_state = self.breaker_state.value


        status: str | Unset = UNSET
        if not isinstance(self.status, Unset):
            status = self.status.value


        uptime_sec = self.uptime_sec

        version = self.version


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
        })
        if breaker_state is not UNSET:
            field_dict["breaker_state"] = breaker_state
        if status is not UNSET:
            field_dict["status"] = status
        if uptime_sec is not UNSET:
            field_dict["uptime_sec"] = uptime_sec
        if version is not UNSET:
            field_dict["version"] = version

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        _breaker_state = d.pop("breaker_state", UNSET)
        breaker_state: GetHealthzResponse200BreakerState | Unset
        if isinstance(_breaker_state,  Unset):
            breaker_state = UNSET
        else:
            breaker_state = GetHealthzResponse200BreakerState(_breaker_state)




        _status = d.pop("status", UNSET)
        status: GetHealthzResponse200Status | Unset
        if isinstance(_status,  Unset):
            status = UNSET
        else:
            status = GetHealthzResponse200Status(_status)




        uptime_sec = d.pop("uptime_sec", UNSET)

        version = d.pop("version", UNSET)

        get_healthz_response_200 = cls(
            breaker_state=breaker_state,
            status=status,
            uptime_sec=uptime_sec,
            version=version,
        )


        get_healthz_response_200.additional_properties = d
        return get_healthz_response_200

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

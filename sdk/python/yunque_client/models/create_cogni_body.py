from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast

if TYPE_CHECKING:
  from ..models.create_cogni_body_activation import CreateCogniBodyActivation
  from ..models.create_cogni_body_checks_item import CreateCogniBodyChecksItem
  from ..models.create_cogni_body_context import CreateCogniBodyContext
  from ..models.create_cogni_body_economics import CreateCogniBodyEconomics
  from ..models.create_cogni_body_experience import CreateCogniBodyExperience
  from ..models.create_cogni_body_mcp import CreateCogniBodyMcp
  from ..models.create_cogni_body_memory import CreateCogniBodyMemory
  from ..models.create_cogni_body_surface import CreateCogniBodySurface
  from ..models.create_cogni_body_workflows_item import CreateCogniBodyWorkflowsItem





T = TypeVar("T", bound="CreateCogniBody")



@_attrs_define
class CreateCogniBody:
    """ Cogni declaration. Same shape as a cogni.yaml file — see pkg/cogni for the full struct.

        Attributes:
            id (str): Unique Cogni id (also used as filename).
            activation (CreateCogniBodyActivation | Unset): ActivationRules (when this Cogni engages).
            capsule (str | Unset): Capsule (persona) this Cogni binds to. Empty for free-standing routing policies.
            checks (list[CreateCogniBodyChecksItem] | Unset): Activation self-tests.
            context (CreateCogniBodyContext | Unset): ContextInjection (extra text added to the system prompt).
            description (str | Unset):
            display_name (str | Unset):
            economics (CreateCogniBodyEconomics | Unset): Per-Cogni budget / cost limits.
            exclusive (str | Unset): If non-empty, only one Cogni with this exclusive-group may activate per turn.
            experience (CreateCogniBodyExperience | Unset):
            mcp (CreateCogniBodyMcp | Unset): MCPConfig (per-Cogni MCP server connections + tool filters).
            memory (CreateCogniBodyMemory | Unset):
            priority (int | Unset): Tie-break multiple activated Cognis (lower = higher priority). Default: 100.
            surface (CreateCogniBodySurface | Unset): ToolSurface (which tools/capabilities are exposed).
            workflows (list[CreateCogniBodyWorkflowsItem] | Unset): Multi-step workflows.
     """

    id: str
    activation: CreateCogniBodyActivation | Unset = UNSET
    capsule: str | Unset = UNSET
    checks: list[CreateCogniBodyChecksItem] | Unset = UNSET
    context: CreateCogniBodyContext | Unset = UNSET
    description: str | Unset = UNSET
    display_name: str | Unset = UNSET
    economics: CreateCogniBodyEconomics | Unset = UNSET
    exclusive: str | Unset = UNSET
    experience: CreateCogniBodyExperience | Unset = UNSET
    mcp: CreateCogniBodyMcp | Unset = UNSET
    memory: CreateCogniBodyMemory | Unset = UNSET
    priority: int | Unset = 100
    surface: CreateCogniBodySurface | Unset = UNSET
    workflows: list[CreateCogniBodyWorkflowsItem] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.create_cogni_body_activation import CreateCogniBodyActivation
        from ..models.create_cogni_body_checks_item import CreateCogniBodyChecksItem
        from ..models.create_cogni_body_context import CreateCogniBodyContext
        from ..models.create_cogni_body_economics import CreateCogniBodyEconomics
        from ..models.create_cogni_body_experience import CreateCogniBodyExperience
        from ..models.create_cogni_body_mcp import CreateCogniBodyMcp
        from ..models.create_cogni_body_memory import CreateCogniBodyMemory
        from ..models.create_cogni_body_surface import CreateCogniBodySurface
        from ..models.create_cogni_body_workflows_item import CreateCogniBodyWorkflowsItem
        id = self.id

        activation: dict[str, Any] | Unset = UNSET
        if not isinstance(self.activation, Unset):
            activation = self.activation.to_dict()

        capsule = self.capsule

        checks: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.checks, Unset):
            checks = []
            for checks_item_data in self.checks:
                checks_item = checks_item_data.to_dict()
                checks.append(checks_item)



        context: dict[str, Any] | Unset = UNSET
        if not isinstance(self.context, Unset):
            context = self.context.to_dict()

        description = self.description

        display_name = self.display_name

        economics: dict[str, Any] | Unset = UNSET
        if not isinstance(self.economics, Unset):
            economics = self.economics.to_dict()

        exclusive = self.exclusive

        experience: dict[str, Any] | Unset = UNSET
        if not isinstance(self.experience, Unset):
            experience = self.experience.to_dict()

        mcp: dict[str, Any] | Unset = UNSET
        if not isinstance(self.mcp, Unset):
            mcp = self.mcp.to_dict()

        memory: dict[str, Any] | Unset = UNSET
        if not isinstance(self.memory, Unset):
            memory = self.memory.to_dict()

        priority = self.priority

        surface: dict[str, Any] | Unset = UNSET
        if not isinstance(self.surface, Unset):
            surface = self.surface.to_dict()

        workflows: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.workflows, Unset):
            workflows = []
            for workflows_item_data in self.workflows:
                workflows_item = workflows_item_data.to_dict()
                workflows.append(workflows_item)




        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "id": id,
        })
        if activation is not UNSET:
            field_dict["activation"] = activation
        if capsule is not UNSET:
            field_dict["capsule"] = capsule
        if checks is not UNSET:
            field_dict["checks"] = checks
        if context is not UNSET:
            field_dict["context"] = context
        if description is not UNSET:
            field_dict["description"] = description
        if display_name is not UNSET:
            field_dict["display_name"] = display_name
        if economics is not UNSET:
            field_dict["economics"] = economics
        if exclusive is not UNSET:
            field_dict["exclusive"] = exclusive
        if experience is not UNSET:
            field_dict["experience"] = experience
        if mcp is not UNSET:
            field_dict["mcp"] = mcp
        if memory is not UNSET:
            field_dict["memory"] = memory
        if priority is not UNSET:
            field_dict["priority"] = priority
        if surface is not UNSET:
            field_dict["surface"] = surface
        if workflows is not UNSET:
            field_dict["workflows"] = workflows

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.create_cogni_body_activation import CreateCogniBodyActivation
        from ..models.create_cogni_body_checks_item import CreateCogniBodyChecksItem
        from ..models.create_cogni_body_context import CreateCogniBodyContext
        from ..models.create_cogni_body_economics import CreateCogniBodyEconomics
        from ..models.create_cogni_body_experience import CreateCogniBodyExperience
        from ..models.create_cogni_body_mcp import CreateCogniBodyMcp
        from ..models.create_cogni_body_memory import CreateCogniBodyMemory
        from ..models.create_cogni_body_surface import CreateCogniBodySurface
        from ..models.create_cogni_body_workflows_item import CreateCogniBodyWorkflowsItem
        d = dict(src_dict)
        id = d.pop("id")

        _activation = d.pop("activation", UNSET)
        activation: CreateCogniBodyActivation | Unset
        if isinstance(_activation,  Unset):
            activation = UNSET
        else:
            activation = CreateCogniBodyActivation.from_dict(_activation)




        capsule = d.pop("capsule", UNSET)

        _checks = d.pop("checks", UNSET)
        checks: list[CreateCogniBodyChecksItem] | Unset = UNSET
        if _checks is not UNSET:
            checks = []
            for checks_item_data in _checks:
                checks_item = CreateCogniBodyChecksItem.from_dict(checks_item_data)



                checks.append(checks_item)


        _context = d.pop("context", UNSET)
        context: CreateCogniBodyContext | Unset
        if isinstance(_context,  Unset):
            context = UNSET
        else:
            context = CreateCogniBodyContext.from_dict(_context)




        description = d.pop("description", UNSET)

        display_name = d.pop("display_name", UNSET)

        _economics = d.pop("economics", UNSET)
        economics: CreateCogniBodyEconomics | Unset
        if isinstance(_economics,  Unset):
            economics = UNSET
        else:
            economics = CreateCogniBodyEconomics.from_dict(_economics)




        exclusive = d.pop("exclusive", UNSET)

        _experience = d.pop("experience", UNSET)
        experience: CreateCogniBodyExperience | Unset
        if isinstance(_experience,  Unset):
            experience = UNSET
        else:
            experience = CreateCogniBodyExperience.from_dict(_experience)




        _mcp = d.pop("mcp", UNSET)
        mcp: CreateCogniBodyMcp | Unset
        if isinstance(_mcp,  Unset):
            mcp = UNSET
        else:
            mcp = CreateCogniBodyMcp.from_dict(_mcp)




        _memory = d.pop("memory", UNSET)
        memory: CreateCogniBodyMemory | Unset
        if isinstance(_memory,  Unset):
            memory = UNSET
        else:
            memory = CreateCogniBodyMemory.from_dict(_memory)




        priority = d.pop("priority", UNSET)

        _surface = d.pop("surface", UNSET)
        surface: CreateCogniBodySurface | Unset
        if isinstance(_surface,  Unset):
            surface = UNSET
        else:
            surface = CreateCogniBodySurface.from_dict(_surface)




        _workflows = d.pop("workflows", UNSET)
        workflows: list[CreateCogniBodyWorkflowsItem] | Unset = UNSET
        if _workflows is not UNSET:
            workflows = []
            for workflows_item_data in _workflows:
                workflows_item = CreateCogniBodyWorkflowsItem.from_dict(workflows_item_data)



                workflows.append(workflows_item)


        create_cogni_body = cls(
            id=id,
            activation=activation,
            capsule=capsule,
            checks=checks,
            context=context,
            description=description,
            display_name=display_name,
            economics=economics,
            exclusive=exclusive,
            experience=experience,
            mcp=mcp,
            memory=memory,
            priority=priority,
            surface=surface,
            workflows=workflows,
        )


        create_cogni_body.additional_properties = d
        return create_cogni_body

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

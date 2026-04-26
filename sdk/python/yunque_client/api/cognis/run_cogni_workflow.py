from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.error import Error
from ...models.run_cogni_workflow_body import RunCogniWorkflowBody
from ...models.run_cogni_workflow_response_200 import RunCogniWorkflowResponse200
from ...types import UNSET, Unset
from typing import cast



def _get_kwargs(
    id: str,
    name: str,
    *,
    body: RunCogniWorkflowBody | Unset = UNSET,

) -> dict[str, Any]:
    headers: dict[str, Any] = {}


    

    

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/v1/cognis/{id}/workflow/{name}".format(id=quote(str(id), safe=""),name=quote(str(name), safe=""),),
    }

    
    if not isinstance(body, Unset):
        _kwargs["json"] = body.to_dict()


    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs



def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Error | RunCogniWorkflowResponse200 | None:
    if response.status_code == 200:
        response_200 = RunCogniWorkflowResponse200.from_dict(response.json())



        return response_200

    if response.status_code == 400:
        response_400 = Error.from_dict(response.json())



        return response_400

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())



        return response_401

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())



        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Error | RunCogniWorkflowResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: str,
    name: str,
    *,
    client: AuthenticatedClient,
    body: RunCogniWorkflowBody | Unset = UNSET,

) -> Response[Error | RunCogniWorkflowResponse200]:
    """ Run a named workflow on one Cogni

    Args:
        id (str):
        name (str):
        body (RunCogniWorkflowBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | RunCogniWorkflowResponse200]
     """


    kwargs = _get_kwargs(
        id=id,
name=name,
body=body,

    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)

def sync(
    id: str,
    name: str,
    *,
    client: AuthenticatedClient,
    body: RunCogniWorkflowBody | Unset = UNSET,

) -> Error | RunCogniWorkflowResponse200 | None:
    """ Run a named workflow on one Cogni

    Args:
        id (str):
        name (str):
        body (RunCogniWorkflowBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | RunCogniWorkflowResponse200
     """


    return sync_detailed(
        id=id,
name=name,
client=client,
body=body,

    ).parsed

async def asyncio_detailed(
    id: str,
    name: str,
    *,
    client: AuthenticatedClient,
    body: RunCogniWorkflowBody | Unset = UNSET,

) -> Response[Error | RunCogniWorkflowResponse200]:
    """ Run a named workflow on one Cogni

    Args:
        id (str):
        name (str):
        body (RunCogniWorkflowBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | RunCogniWorkflowResponse200]
     """


    kwargs = _get_kwargs(
        id=id,
name=name,
body=body,

    )

    response = await client.get_async_httpx_client().request(
        **kwargs
    )

    return _build_response(client=client, response=response)

async def asyncio(
    id: str,
    name: str,
    *,
    client: AuthenticatedClient,
    body: RunCogniWorkflowBody | Unset = UNSET,

) -> Error | RunCogniWorkflowResponse200 | None:
    """ Run a named workflow on one Cogni

    Args:
        id (str):
        name (str):
        body (RunCogniWorkflowBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | RunCogniWorkflowResponse200
     """


    return (await asyncio_detailed(
        id=id,
name=name,
client=client,
body=body,

    )).parsed

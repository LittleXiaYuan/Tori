from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.error import Error
from ...models.post_api_providers_switch_model_body import PostApiProvidersSwitchModelBody
from ...models.post_api_providers_switch_model_response_200 import PostApiProvidersSwitchModelResponse200
from ...types import UNSET, Unset
from typing import cast



def _get_kwargs(
    *,
    body: PostApiProvidersSwitchModelBody | Unset = UNSET,

) -> dict[str, Any]:
    headers: dict[str, Any] = {}


    

    

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/providers/switch-model",
    }

    
    if not isinstance(body, Unset):
        _kwargs["json"] = body.to_dict()


    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs



def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Error | PostApiProvidersSwitchModelResponse200 | None:
    if response.status_code == 200:
        response_200 = PostApiProvidersSwitchModelResponse200.from_dict(response.json())



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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Error | PostApiProvidersSwitchModelResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: PostApiProvidersSwitchModelBody | Unset = UNSET,

) -> Response[Error | PostApiProvidersSwitchModelResponse200]:
    """ POST /api/providers/switch-model

    Args:
        body (PostApiProvidersSwitchModelBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | PostApiProvidersSwitchModelResponse200]
     """


    kwargs = _get_kwargs(
        body=body,

    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)

def sync(
    *,
    client: AuthenticatedClient,
    body: PostApiProvidersSwitchModelBody | Unset = UNSET,

) -> Error | PostApiProvidersSwitchModelResponse200 | None:
    """ POST /api/providers/switch-model

    Args:
        body (PostApiProvidersSwitchModelBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | PostApiProvidersSwitchModelResponse200
     """


    return sync_detailed(
        client=client,
body=body,

    ).parsed

async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: PostApiProvidersSwitchModelBody | Unset = UNSET,

) -> Response[Error | PostApiProvidersSwitchModelResponse200]:
    """ POST /api/providers/switch-model

    Args:
        body (PostApiProvidersSwitchModelBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | PostApiProvidersSwitchModelResponse200]
     """


    kwargs = _get_kwargs(
        body=body,

    )

    response = await client.get_async_httpx_client().request(
        **kwargs
    )

    return _build_response(client=client, response=response)

async def asyncio(
    *,
    client: AuthenticatedClient,
    body: PostApiProvidersSwitchModelBody | Unset = UNSET,

) -> Error | PostApiProvidersSwitchModelResponse200 | None:
    """ POST /api/providers/switch-model

    Args:
        body (PostApiProvidersSwitchModelBody | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | PostApiProvidersSwitchModelResponse200
     """


    return (await asyncio_detailed(
        client=client,
body=body,

    )).parsed

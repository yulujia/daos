# Pool Service

A pool service maintains pool metadata, including:

- **Pool connections**. Each pool connection is represented by a pool handle identified by a client-generated handle UUID. See POOL_CONNECT below. The terms �pool connection� and �pool handle� are used interchangeably in this document.
- **Pool map**. This is a versioned data structure recording the set of targets belonging to this pool, the fault domains to which the targets belong, and the status of the targets. Versioning (which is unrelated to epochs) facilitates lazy dissemination of the pool map across clients and targets. See <a href="../object/key_array_object.md#10.1">*Pool Map*</a>.
- **Pool name space**. This provides a pool-wise, flat name space of string-based, user-friendly container names. Looking up a container name in the pool name space gets the corresponding container UUID.
- **Container index**. This maps a container UUID to the ID of the corresponding container service.
- **Container service index**. This maps a container service ID to the addresses of the corresponding container service replicas.
- **Security information**. Attributes like the UID, the GID, and the mode.
- **Upper-layer pool metadata**. Attributes used by the DAOS-SR layer or layers even higher above.

A pool service handles the following RPC procedures. �pool_handle_uuid� is a UUID identifying a pool handle.

- **POOL_CONNECT**(pool_uuid, pool_handle_uuid, authenticator, capabilities) (error, pool_map). Establish a pool handle/connection. �pool_uuid� identifies the pool to connect to. �pool_handle_uuid� is generated by the client. �authenticator� contains information (e.g., the UID and the GID) required by the authentication scheme as well as a client process set identifier used by the server side for access restriction and the pool handle eviction. �capabilities� indicates the access rights (e.g., read-only or read-write) requested by the client. �pool_map� is a packed representation of the pool map, returned to the client if the pool handle is authorized.
- **POOL_DISCONNECT**(pool_handle_uuid) error. Close a pool handle/connection.
- **POOL_QUERY**(pool_handle_uuid) (error, pool_state). Query various information (e.g., size, free space, etc. returned through �pool_state�) about a pool.
- **POOL_TARGET_ADD**(pool_handle_uuid, targets) error. Add new targets to a pool. �targets� is the set of the addresses of the new targets.
- **POOL_TARGET_DISABLE**(pool_handle_uuid, targets) error. Disable existing targets in a pool. �targets� is the set of the addresses of the targets to disable.
- **POOL_CONTAINER_CREATE**(pool_handle_uuid, container_uuid, name) (error, container_addresses). Create a container with a UUID generated by the client and optional a name if �name� passes in a string.
- **POOL_CONTAINER_DESTROY**(pool_handle_uuid, container_uuid, name) error. Destroy a container identified by either UUID or name. If the container has a name, �name� shall be specified; otherwise �container_uuid� shall be specified and �name� shall be left unspecified.
- **POOL_CONTAINER_LOOKUP**(pool_handle_uuid, name) (error, container_addresses). Look up a container by name.
- **POOL_CONTAINER_RENAME**(pool_handle_uuid, from_name, container_uuid, to_name) error. Rename a container. If the container does not have an existing name, �container_uuid� shall be specified instead of �from_name�, allowing a name to be add to an anonymous container.
- **POOL_EPOCH_COMMIT**(pool_handle_uuid, container_handle_uuids, epochs) error. Commit a set of epochs in different containers atomically. �container_handle_uuids� is a list of the handle UUIDs of the containers. �epochs� is a list of the epochs. For any index i, epochs[i] is an epoch in the container referred to by container_handle_uuids[i].

The target service handles the following RPC procedures in addition to those required by I/O bypasses:

- **TARGET_CONNECT**(pool_uuid, pool_handle_uuid, authenticator, capabilities) error. Establish a pool handle/connection authorized by the pool service on this target. Sent only by the pool service in response to a **POOL_CONNECT** request. �authenticator� contains information (e.g., encrypted using a key shared only among the targets) required to verify the request comes from a target rather than a client. For implementations restricting a pool handle to a client process set, the client process set identifier can also be passed through �authenticator�.
- **TARGET_DISCONNECT**(pool_handle_uuid) error. Close a pool handle/connection on this target. Sent only by the pool service in response to a **POOL_DISCONNECT** request.
- **TARGET_QUERY**(pool_handle_uuid) (error, target_state). Query various information (e.g., size, space usage, storage type, etc. returned through �target_state�) of a target.

<a id="8.4"></a>
## Pool Creation

Because creating a pool requires special privileges for steps related to storage allocation and fault domain querying, it is handled by the storage management module, as described in <a href="../client/layering.md#56">*Integration with System Management*</a> and <a href="/doc/use_cases.md#61">*Storage Management and Workflow Integration*</a>. After the target formatting is done, the storage management module calls DSM with the list of targets and their fault domains, to create and initialize a pool service. DSM creates the pool service following the principle described in *<a href="#8.3.3">Service Management</a>*. The list of targets and their fault domains are then converted into the initial version of the pool map and stored in the pool service, along with other initial pool metadata. When returning to the storage management module, DSM reports back the address of the pool service, which will eventually be passed to the application and used to address the POOL_CONNECT RPC(s).

<a id="8.5"></a>
## Pool Connections

To establish a pool connection, a client process calls the pool connect method in the client library with the pool UUID, the pool service address, and the requested capabilities. The client library sends a POOL_CONNECT request to the pool service. The pool service tries to authenticate the request using the authenticator, as defined by the security model (e.g., UID/GID in a POSIX-like model), and to authorize the requested capabilities to the client-generated pool handle UUID. If everything goes well, the pool service sends a collective TARGET_CONNECT request to all targets in the pool, with the pool handle UUID, an optional authenticator, and the granted capabilities. This authenticator also passes in the identifier of the client process set, so that the server side may restrict the pool connection to members of the same client process set. After the collective request completes successfully, the pool service replies to the client library with the pool map. The client process may then pack, transfer, and unpack the resulting connection context to its peers, using the utility methods described in *<a href="#8.1">Client Library</a>*.

To destroy a pool connection, a client process calls the pool disconnect method in the client library with the pool handle, triggering a POOL_DISCONNECT request to the pool service. The pool service sends a collective TARGET_DISCONNECT request to all targets in the pool and replies to the client library once the collective request completes. These steps destroy all state associated with the connection, including all container handles. Other client processes sharing this connection should destroy their copies of the pool handle locally, preferably before the disconnect method is called on behalf of everyone. If a group of client processes terminate prematurely, before having a chance to call the pool disconnect method, their pool connection will eventually be evicted once the pool service learns about the event from the run-time environment, using the corresponding client process set identifier.
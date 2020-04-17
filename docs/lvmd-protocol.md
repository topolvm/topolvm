# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [lvmd/proto/lvmd.proto](#lvmd/proto/lvmd.proto)
    - [CreateLVRequest](#proto.CreateLVRequest)
    - [CreateLVResponse](#proto.CreateLVResponse)
    - [Empty](#proto.Empty)
    - [GetFreeBytesRequest](#proto.GetFreeBytesRequest)
    - [GetFreeBytesResponse](#proto.GetFreeBytesResponse)
    - [GetLVListRequest](#proto.GetLVListRequest)
    - [GetLVListResponse](#proto.GetLVListResponse)
    - [LogicalVolume](#proto.LogicalVolume)
    - [RemoveLVRequest](#proto.RemoveLVRequest)
    - [ResizeLVRequest](#proto.ResizeLVRequest)
    - [WatchItem](#proto.WatchItem)
    - [WatchResponse](#proto.WatchResponse)
  
  
  
    - [LVService](#proto.LVService)
    - [VGService](#proto.VGService)
  

- [Scalar Value Types](#scalar-value-types)



<a name="lvmd/proto/lvmd.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## lvmd/proto/lvmd.proto
LVMd manages logical volumes of an LVM volume group.

The protocol consists of two services:
- VGService provides information of the volume group.
- LVService provides management functions for logical volumes on the volume group.


<a name="proto.CreateLVRequest"></a>

### CreateLVRequest
Represents the input for CreateLV.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | The logical volume name. |
| size_gb | [uint64](#uint64) |  | Volume size in GiB. |
| tags | [string](#string) | repeated | Tags to add to the volume during creation |
| vg_name | [string](#string) |  |  |






<a name="proto.CreateLVResponse"></a>

### CreateLVResponse
Represents the response of CreateLV.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volume | [LogicalVolume](#proto.LogicalVolume) |  | Information of the created volume. |






<a name="proto.Empty"></a>

### Empty







<a name="proto.GetFreeBytesRequest"></a>

### GetFreeBytesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| vg_name | [string](#string) |  |  |






<a name="proto.GetFreeBytesResponse"></a>

### GetFreeBytesResponse
Represents the response of GetFreeBytes.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| free_bytes | [uint64](#uint64) |  | Free space of the volume group in bytes. |






<a name="proto.GetLVListRequest"></a>

### GetLVListRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| vg_name | [string](#string) |  |  |






<a name="proto.GetLVListResponse"></a>

### GetLVListResponse
Represents the response of GetLVList.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| volumes | [LogicalVolume](#proto.LogicalVolume) | repeated | Information of volumes. |






<a name="proto.LogicalVolume"></a>

### LogicalVolume
Represents a logical volume.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | The logical volume name. |
| size_gb | [uint64](#uint64) |  | Volume size in GiB. |
| dev_major | [uint32](#uint32) |  | Device major number. |
| dev_minor | [uint32](#uint32) |  | Device minor number. |
| tags | [string](#string) | repeated | Tags to add to the volume during creation |
| vg_name | [string](#string) |  |  |






<a name="proto.RemoveLVRequest"></a>

### RemoveLVRequest
Represents the input for RemoveLV.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | The logical volume name. |
| vg_name | [string](#string) |  |  |






<a name="proto.ResizeLVRequest"></a>

### ResizeLVRequest
Represents the input for ResizeLV.

The volume must already exist.
The volume size will be set to exactly &#34;size_gb&#34;.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | The logical volume name. |
| size_gb | [uint64](#uint64) |  | Volume size in GiB. |
| vg_name | [string](#string) |  |  |






<a name="proto.WatchItem"></a>

### WatchItem



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| free_bytes | [uint64](#uint64) |  | Free space of the volume group in bytes. |
| vg_name | [string](#string) |  |  |






<a name="proto.WatchResponse"></a>

### WatchResponse
Represents the stream output from Watch.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| items | [WatchItem](#proto.WatchItem) | repeated |  |





 

 

 


<a name="proto.LVService"></a>

### LVService
Service to manage logical volumes of the volume group.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateLV | [CreateLVRequest](#proto.CreateLVRequest) | [CreateLVResponse](#proto.CreateLVResponse) | Create a logical volume. |
| RemoveLV | [RemoveLVRequest](#proto.RemoveLVRequest) | [Empty](#proto.Empty) | Remove a logical volume. |
| ResizeLV | [ResizeLVRequest](#proto.ResizeLVRequest) | [Empty](#proto.Empty) | Resize a logical volume. |


<a name="proto.VGService"></a>

### VGService
Service to retrieve information of the volume group.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetLVList | [GetLVListRequest](#proto.GetLVListRequest) | [GetLVListResponse](#proto.GetLVListResponse) | Get the list of logical volumes in the volume group. |
| GetFreeBytes | [GetFreeBytesRequest](#proto.GetFreeBytesRequest) | [GetFreeBytesResponse](#proto.GetFreeBytesResponse) | Get the free space of the volume group in bytes. |
| Watch | [Empty](#proto.Empty) | [WatchResponse](#proto.WatchResponse) stream | Stream the volume group metrics. |

 



## Scalar Value Types

| .proto Type | Notes | C++ Type | Java Type | Python Type |
| ----------- | ----- | -------- | --------- | ----------- |
| <a name="double" /> double |  | double | double | float |
| <a name="float" /> float |  | float | float | float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long |
| <a name="bool" /> bool |  | bool | boolean | boolean |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str |


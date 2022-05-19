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
    - [ThinPoolItem](#proto.ThinPoolItem)
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
| device_class | [string](#string) |  |  |






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
| device_class | [string](#string) |  |  |






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
| device_class | [string](#string) |  |  |






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






<a name="proto.RemoveLVRequest"></a>

### RemoveLVRequest
Represents the input for RemoveLV.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | The logical volume name. |
| device_class | [string](#string) |  |  |






<a name="proto.ResizeLVRequest"></a>

### ResizeLVRequest
Represents the input for ResizeLV.

The volume must already exist.
The volume size will be set to exactly &#34;size_gb&#34;.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | The logical volume name. |
| size_gb | [uint64](#uint64) |  | Volume size in GiB. |
| device_class | [string](#string) |  |  |






<a name="proto.ThinPoolItem"></a>

### ThinPoolItem
Represents the details of thinpool.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data_percent | [double](#double) |  | Data percent occupied on the thinpool, used for monitoring. |
| metadata_percent | [double](#double) |  | Metadata percent occupied on the thinpool, used for monitoring. |
| overprovision_bytes | [uint64](#uint64) |  | Free space on the thinpool with overprovision, used for annotating node. |
| size_bytes | [uint64](#uint64) |  | Physical data space size of the thinpool. |






<a name="proto.WatchItem"></a>

### WatchItem
Represents the response corresponding to device class targets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| free_bytes | [uint64](#uint64) |  | Free space in the volume group in bytes. |
| device_class | [string](#string) |  |  |
| size_bytes | [uint64](#uint64) |  | Size of volume group in bytes. |
| thin_pool | [ThinPoolItem](#proto.ThinPoolItem) |  |  |






<a name="proto.WatchResponse"></a>

### WatchResponse
Represents the stream output from Watch.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| free_bytes | [uint64](#uint64) |  | Free space of the default volume group in bytes. |
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

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |


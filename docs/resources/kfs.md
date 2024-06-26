---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "kowabunga_kfs Resource - terraform-provider-kowabunga"
subcategory: ""
description: |-
  Manages a KFS distributed network storage resource. KFS (stands for Kowabunga File System) provides an elastic NFS-compatible endpoint.
---

# kowabunga_kfs (Resource)

Manages a KFS distributed network storage resource. **KFS** (stands for *Kowabunga File System*) provides an elastic NFS-compatible endpoint.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Resource name
- `project` (String) Associated project name or ID
- `region` (String) Associated region name or ID

### Optional

- `access_type` (String) KFS' access type. Allowed values: 'RW' or 'RO'. Defaults to RW.
- `desc` (String) Resource extended description
- `nfs` (String) Associated NFS storage name or ID (zone's default if unspecified)
- `protocols` (List of Number) KFS's requested NFS protocols versions (defaults to NFSv3 and NFSv4))
- `timeouts` (Attributes) (see [below for nested schema](#nestedatt--timeouts))

### Read-Only

- `endpoint` (String) NFS Endoint (read-only)
- `id` (String) Resource object internal identifier

<a id="nestedatt--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) 30m0s
- `delete` (String) 5m0s
- `read` (String) 2m0s
- `update` (String) 5m0s

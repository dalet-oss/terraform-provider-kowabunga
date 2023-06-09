---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "kowabunga_storage_pool Resource - terraform-provider-kowabunga"
subcategory: ""
description: |-
  Manages a storage pool resource
---

# kowabunga_storage_pool (Resource)

Manages a storage pool resource



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Resource name
- `pool` (String) Ceph RBD pool name
- `zone` (String) Associated zone name or ID

### Optional

- `address` (String) Ceph RBD monitor address or hostname
- `currency` (String) libvirt host monthly price currency (default: **EUR**)
- `default` (Boolean) Whether to set pool as zone's default one (default: **false**). First pool to be created is always considered as default's one.
- `desc` (String) Resource extended description
- `host` (String) Host to bind the storage pool to (default: none)
- `port` (Number) Ceph RBD monitor port number
- `price` (Number) libvirt host monthly price value (default: 0)
- `secret` (String, Sensitive) CephX client authentication UUID
- `type` (String) Storage pool type ('local' or 'rbd', defaults to 'rbd')

### Read-Only

- `id` (String) Resource object internal identifier



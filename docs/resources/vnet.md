---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "kowabunga_vnet Resource - terraform-provider-kowabunga"
subcategory: ""
description: |-
  Manages a virtual network resource
---

# kowabunga_vnet (Resource)

Manages a virtual network resource



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `interface` (String) Host bridge network interface
- `name` (String) Resource name
- `region` (String) Associated region name or ID
- `vlan` (Number) VLAN ID

### Optional

- `desc` (String) Resource extended description
- `private` (Boolean) Whether the virtual network is private or public (default: **true**, i.e. private). The first virtual network to be created is always considered to be the default one.
- `timeouts` (Attributes) (see [below for nested schema](#nestedatt--timeouts))

### Read-Only

- `id` (String) Resource object internal identifier

<a id="nestedatt--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) 30m0s
- `delete` (String) 5m0s
- `read` (String) 2m0s
- `update` (String) 5m0s

---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "kowabunga_kompute Resource - terraform-provider-kowabunga"
subcategory: ""
description: |-
  Manages a Kompute virtual machine resource. Kompute is an seamless automated way to create virtual machine resources. It abstract the complexity of manually creating instance, volumes and network adapters resources and binding them together. It is the RECOMMENDED way to create and manipulate virtual machine services, unless a specific hwardware configuration is required. Kompute provides 2 network adapters, a public (WAN) and a private (LAN/VPC) one, as well as up to two disks (first one for OS, optional second one for extra data).
---

# kowabunga_kompute (Resource)

Manages a Kompute virtual machine resource. **Kompute** is an seamless automated way to create virtual machine resources. It abstract the complexity of manually creating instance, volumes and network adapters resources and binding them together. It is the **RECOMMENDED** way to create and manipulate virtual machine services, unless a specific hwardware configuration is required. Kompute provides 2 network adapters, a public (WAN) and a private (LAN/VPC) one, as well as up to two disks (first one for OS, optional second one for extra data).



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `disk` (Number) The Kompute instance OS disk size (expressed in GB)
- `mem` (Number) The Kompute instance memory size (expressed in GB)
- `name` (String) Resource name
- `project` (String) Associated project name or ID
- `vcpus` (Number) The Kompute instance number of vCPUs
- `zone` (String) Associated zone name or ID

### Optional

- `desc` (String) Resource extended description
- `extra_disk` (Number) The Kompute optional data disk size (expressed in GB, disabled by default, 0 to disable)
- `pool` (String) Associated storage pool name or ID (zone's default if unspecified)
- `public` (Boolean) Should Kompute instance be exposed over public Internet ? (default: **false**)
- `template` (String) Associated template name or ID (zone's default storage pool's default if unspecified)
- `timeouts` (Attributes) (see [below for nested schema](#nestedatt--timeouts))

### Read-Only

- `id` (String) Resource object internal identifier
- `ip` (String) IP (read-only)

<a id="nestedatt--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) 30m0s
- `delete` (String) 5m0s
- `read` (String) 2m0s
- `update` (String) 5m0s
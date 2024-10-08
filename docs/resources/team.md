---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "kowabunga_team Resource - terraform-provider-kowabunga"
subcategory: ""
description: |-
  Manages a Kowabunga team resource
---

# kowabunga_team (Resource)

Manages a Kowabunga team resource



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Resource name
- `users` (List of String) The list of users to be associated with the instance

### Optional

- `desc` (String) Resource extended description
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

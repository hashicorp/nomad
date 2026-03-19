/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import can from 'ember-can/helpers/can';
import editableVariableLink from 'nomad-ui/helpers/editable-variable-link';
import { HdsLinkInline } from '@hashicorp/design-system-components/components';

export const EditableVariableLink = <template>
  {{! Either link to a new variable with a pre-filled path, or the existing variable in edit mode, depending if it exists }}
  {{#if (can "write variable")}}
    {{#let
      (editableVariableLink
        @path existingPaths=@existingPaths namespace=@namespace
      )
      as |link|
    }}
      {{#if link.model}}
        <HdsLinkInline
          @route={{link.route}}
          @model={{link.model}}
          @query={{link.query}}
        >{{@path}}</HdsLinkInline>
      {{else}}
        <HdsLinkInline
          @route={{link.route}}
          @query={{link.query}}
        >{{@path}}</HdsLinkInline>
      {{/if}}
    {{/let}}
  {{else}}
    {{@path}}
  {{/if}}
</template>;

export default EditableVariableLink;

/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { service } from '@ember/service';
import {
  HdsIcon,
  HdsTable,
} from '@hashicorp/design-system-components/components';
import can from 'ember-can/helpers/can';
import momentFromNow from 'ember-moment/helpers/moment-from-now';
import formatTs from 'nomad-ui/helpers/format-ts';
import trimPath from 'nomad-ui/helpers/trim-path';
import keyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import compactPath from '../utils/compact-path';

export default class VariablePaths extends Component {
  @service router;
  @service abilities;

  get folders() {
    return Object.entries(this.args.branch.children).map(([name]) => {
      return compactPath(this.args.branch.children[name], name);
    });
  }

  get files() {
    return this.args.branch.files;
  }

  handleFolderClick = async (path, trigger) => {
    // Don't navigate if the user clicked on a link; this happens with cmd/ctrl-click on the link itself.
    if (
      trigger instanceof PointerEvent &&
      /** @type {HTMLElement} */ (trigger.target).tagName === 'A'
    ) {
      return;
    }
    this.router.transitionTo('variables.path', path);
  };

  handleFileClick = async ({ path, variable: { id, namespace } }, trigger) => {
    if (this.abilities.can('read variable', null, { path, namespace })) {
      // Don't navigate if the user clicked on a link; this happens with cmd/ctrl-click on the link itself.
      if (
        trigger instanceof PointerEvent &&
        /** @type {HTMLElement} */ (trigger.target).tagName === 'A'
      ) {
        return;
      }
      this.router.transitionTo('variables.variable', id);
    }
  };

  <template>
    <HdsTable @caption="A list variables" class="path-tree">
      <:head as |H|>
        <H.Tr>
          <H.Th>
            Path
          </H.Th>
          <H.Th>
            Namespace
          </H.Th>
          <H.Th>
            Last Modified
          </H.Th>
        </H.Tr>
      </:head>
      <:body as |B|>
        {{#each this.folders as |folder|}}
          <B.Tr
            data-test-folder-row
            {{on "click" (fn this.handleFolderClick folder.data.absolutePath)}}
          >
            <B.Td
              colspan="3"
              {{keyboardShortcutModifier
                enumerated=true
                action=(fn this.handleFolderClick folder.data.absolutePath)
              }}
            >
              <span>
                <HdsIcon @name="folder" @isInline={{true}} />
                <LinkTo
                  @route="variables.path"
                  @model={{folder.data.absolutePath}}
                  @query={{hash namespace="*"}}
                >
                  {{trimPath folder.name}}
                </LinkTo>
              </span>
            </B.Td>
          </B.Tr>

        {{/each}}

        {{#each this.files as |file|}}
          <B.Tr
            data-test-file-row={{file.name}}
            {{on "click" (fn this.handleFileClick file)}}
            class={{if
              (can
                "read variable"
                path=file.absoluteFilePath
                namespace=file.variable.namespace
              )
              ""
              "inaccessible"
            }}
            {{keyboardShortcutModifier
              enumerated=true
              action=(fn this.handleFileClick file)
            }}
          >
            <B.Td>
              <HdsIcon @name="file-text" @isInline={{true}} />
              {{#if
                (can
                  "read variable"
                  path=file.absoluteFilePath
                  namespace=file.variable.namespace
                )
              }}
                <LinkTo
                  @route="variables.variable"
                  @model={{file.variable.id}}
                  @query={{hash namespace="*"}}
                >
                  {{file.name}}
                </LinkTo>
              {{else}}
                <span
                  title="Your access policy does not allow you to view the contents of {{file.name}}"
                >{{file.name}}</span>
              {{/if}}
            </B.Td>
            <B.Td>
              {{file.variable.namespace}}
            </B.Td>
            <B.Td>
              <span
                class="tooltip"
                aria-label="{{formatTs file.variable.modifyTime}}"
              >
                {{momentFromNow file.variable.modifyTime}}
              </span>
            </B.Td>
          </B.Tr>
        {{/each}}
      </:body>
    </HdsTable>
  </template>
}

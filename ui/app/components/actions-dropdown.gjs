/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { array, concat, fn, get, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { eq } from 'ember-truth-helpers';
import { objectAt } from '@nullvoxpopuli/ember-composable-helpers';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsDropdown,
  HdsReveal,
} from '@hashicorp/design-system-components/components';
import { service } from '@ember/service';

export default class ActionsDropdown extends Component {
  @service nomadActions;
  @service notifications;

  /**
   * @param {HTMLElement} el
   */
  openActionsDropdown = (el) => {
    const dropdownTrigger = el?.getElementsByTagName('button')[0];
    if (dropdownTrigger) {
      dropdownTrigger.click();
    }
  };

  <template>
    <HdsDropdown
      class="actions-dropdown"
      {{keyboardShortcut
        label="Open Actions Dropdown"
        pattern=(array "a" "c")
        action=this.openActionsDropdown
      }}
      as |dd|
    >
      <dd.ToggleButton
        class="action-toggle-button"
        @color="secondary"
        @text="Actions{{if @context (concat ' for ' @context.name)}}"
        @size="medium"
      />
      {{#each @actions as |actionC|}}
        {{#if @allocation}}
          <dd.Interactive
            {{keyboardShortcut
              enumerated=true
              action=(fn
                this.nomadActions.runAction
                (hash action=actionC allocID=@allocation.id)
              )
            }}
            {{on
              "click"
              (fn
                this.nomadActions.runAction
                (hash action=actionC allocID=@allocation.id)
              )
            }}
            @text={{actionC.name}}
          />
        {{else if (eq actionC.allocations.length 1)}}
          <dd.Interactive
            {{keyboardShortcut
              enumerated=true
              action=(fn
                this.nomadActions.runAction
                (hash
                  action=actionC
                  allocID=(get (objectAt 0 actionC.allocations) "id")
                )
              )
            }}
            {{on
              "click"
              (fn
                this.nomadActions.runAction
                (hash
                  action=actionC
                  allocID=(get (objectAt 0 actionC.allocations) "id")
                )
              )
            }}
            @text={{actionC.name}}
          />
        {{else}}
          <dd.Generic>
            <HdsReveal @text={{actionC.name}}>
              <dd.Interactive
                {{keyboardShortcut
                  enumerated=true
                  action=(fn this.nomadActions.runActionOnRandomAlloc actionC)
                }}
                {{on
                  "click"
                  (fn this.nomadActions.runActionOnRandomAlloc actionC)
                }}
                @text="Run on a random alloc"
              />
              <dd.Interactive
                {{keyboardShortcut
                  enumerated=true
                  action=(fn this.nomadActions.runActionOnAllAllocs actionC)
                }}
                {{on
                  "click"
                  (fn this.nomadActions.runActionOnAllAllocs actionC)
                }}
                @text="Run on all {{actionC.allocations.length}} allocs"
              />
            </HdsReveal>
          </dd.Generic>
        {{/if}}
      {{/each}}
    </HdsDropdown>
  </template>
}

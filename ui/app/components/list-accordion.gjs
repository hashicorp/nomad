/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { fn, hash } from '@ember/helper';
import { get } from '@ember/object';
import ListAccordionAccordionBody from 'nomad-ui/components/list-accordion/accordion-body';
import ListAccordionAccordionHead from 'nomad-ui/components/list-accordion/accordion-head';

export default class ListAccordion extends Component {
  @tracked stateCache = [];

  get key() {
    return this.args.key ?? 'id';
  }

  get source() {
    return this.args.source ?? [];
  }

  get startExpanded() {
    return this.args.startExpanded ?? false;
  }

  get decoratedSource() {
    const stateCache = this.stateCache;
    const key = this.key;
    const startExpanded = this.startExpanded;

    return this.source.map((item) => {
      const itemKey = get(item, key);
      const cacheItem = stateCache.find(
        (candidate) => candidate.key === itemKey,
      );

      return {
        item,
        isOpen: cacheItem ? !!cacheItem.isOpen : startExpanded,
      };
    });
  }

  setItemOpenState = (item, isOpen) => {
    const key = this.key;
    const itemKey = get(item, key);
    const nextState = this.stateCache.slice();
    const existingIndex = nextState.findIndex(
      (candidate) => candidate.key === itemKey,
    );

    if (existingIndex === -1) {
      nextState.push({ key: itemKey, isOpen });
    } else {
      nextState[existingIndex] = { key: itemKey, isOpen };
    }

    this.stateCache = nextState;
    this.args.onToggle?.(item, isOpen);
  };

  openItem = (item) => {
    this.setItemOpenState(item, true);
  };

  closeItem = (item) => {
    this.setItemOpenState(item, false);
  };

  <template>
    <div class="accordion" ...attributes>
      {{#each this.decoratedSource as |item|}}
        {{yield
          (hash
            head=(component
              ListAccordionAccordionHead
              isOpen=item.isOpen
              onOpen=(fn this.openItem item.item)
              onClose=(fn this.closeItem item.item)
              item=item.item
            )
            body=(component ListAccordionAccordionBody isOpen=item.isOpen)
            item=item.item
            isOpen=item.isOpen
            onOpen=(fn this.openItem item.item)
            onClose=(fn this.closeItem item.item)
          )
        }}
      {{/each}}
    </div>
  </template>
}

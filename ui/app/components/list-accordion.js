/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed, get } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('accordion')
export default class ListAccordion extends Component {
  key = 'id';
  @overridable(() => []) source;

  onToggle /* item, isOpen */() {}
  startExpanded = false;

  @computed('key', 'source.[]', 'startExpanded', 'stateCache')
  get decoratedSource() {
    const stateCache = this.stateCache;
    const key = this.key;
    const deepKey = `item.${key}`;
    const startExpanded = this.startExpanded;

    const decoratedSource = this.source.map((item) => {
      const cacheItem = stateCache.findBy(deepKey, get(item, key));
      return {
        item,
        isOpen: cacheItem ? !!cacheItem.isOpen : startExpanded,
      };
    });

    // eslint-disable-next-line ember/no-side-effects
    this.set('stateCache', decoratedSource);
    return decoratedSource;
  }

  // When source updates come in, the state cache is used to preserve
  // open/close state.
  @overridable(() => []) stateCache;
}

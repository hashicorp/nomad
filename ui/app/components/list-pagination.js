/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import classic from 'ember-classic-decorator';

@classic
export default class ListPagination extends Component {
  @overridable(() => []) source;
  size = 25;
  page = 1;
  spread = 2;

  @computed('size', 'page')
  get startsAt() {
    return (this.page - 1) * this.size + 1;
  }

  @computed('source.[]', 'size', 'page')
  get endsAt() {
    return Math.min(this.page * this.size, this.get('source.length'));
  }

  @computed('source.[]', 'size')
  get lastPage() {
    return Math.ceil(this.get('source.length') / this.size);
  }

  @computed('source.[]', 'page', 'spread')
  get pageLinks() {
    const { spread, page, lastPage } = this;

    // When there is only one page, don't bother with page links
    if (lastPage === 1) {
      return [];
    }

    const lowerBound = Math.max(1, page - spread);
    const upperBound = Math.min(lastPage, page + spread) + 1;

    return Array(upperBound - lowerBound)
      .fill(null)
      .map((_, index) => ({
        pageNumber: lowerBound + index,
      }));
  }

  @computed('source.[]', 'page', 'size')
  get list() {
    const size = this.size;
    const start = (this.page - 1) * size;
    return this.source.slice(start, start + size);
  }
}

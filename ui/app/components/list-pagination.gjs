/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { hash } from '@ember/helper';
import ListPager from 'nomad-ui/components/list-pagination/list-pager';

export default class ListPagination extends Component {
  get source() {
    return this.args.source ?? [];
  }

  get size() {
    return this.args.size ?? 25;
  }

  get page() {
    return this.args.page ?? 1;
  }

  get spread() {
    return this.args.spread ?? 2;
  }

  get startsAt() {
    return (this.page - 1) * this.size + 1;
  }

  get endsAt() {
    return Math.min(this.page * this.size, this.source.length);
  }

  get lastPage() {
    return Math.ceil(this.source.length / this.size);
  }

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

  get list() {
    const size = this.size;
    const start = (this.page - 1) * size;
    return this.source.slice(start, start + size);
  }

  get firstOrPrevVisible() {
    return this.page !== 1;
  }

  get nextOrLastVisible() {
    return this.page !== this.lastPage;
  }

  get prevPage() {
    return this.page - 1;
  }

  get nextPage() {
    return this.page + 1;
  }

  <template>
    {{#if this.source.length}}
      {{yield
        (hash
          first=(component
            ListPager
            test="first"
            label="First page"
            page=1
            visible=this.firstOrPrevVisible
          )
          prev=(component
            ListPager
            test="prev"
            label="Previous page"
            page=this.prevPage
            visible=this.firstOrPrevVisible
          )
          next=(component
            ListPager
            test="next"
            label="Next page"
            page=this.nextPage
            visible=this.nextOrLastVisible
          )
          last=(component
            ListPager
            test="last"
            label="Last page"
            page=this.lastPage
            visible=this.nextOrLastVisible
          )
          pageLinks=this.pageLinks
          currentPage=this.page
          totalPages=this.lastPage
          startsAt=this.startsAt
          endsAt=this.endsAt
          list=this.list
        )
      }}
    {{/if}}
  </template>
}

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import productMetadata from '../../app/utils/styleguide/product-metadata';

export default {
  title: 'Components/Accordion',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Accordion</h5>
      <ListAccordion @source={{products}} @key="name" as |ac|>
        <ac.head @buttonLabel="details">
          <div class="columns inline-definitions">
            <div class="column is-1">{{ac.item.name}}</div>
            <div class="column is-1">
              <span class="bumper-left badge is-light">{{ac.item.lang}}</span>
            </div>
          </div>
        </ac.head>
        <ac.body>
          <h1 class="title is-4">{{ac.item.name}}</h1>
          <p>{{ac.item.desc}}</p>
          <p><a href="{{ac.item.link}}" target="_parent">Learn more...</a></p>
        </ac.body>
      </ListAccordion>
      `,
    context: {
      products: productMetadata,
    },
  };
};

export let OneItem = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Accordion, one item</h5>
      <ListAccordion @source={{take 1 products}} @key="name" as |a|>
        <a.head @buttonLabel="details">
          <div class="columns inline-definitions">
            <div class="column is-1">{{a.item.name}}</div>
            <div class="column is-1">
              <span class="bumper-left badge is-light">{{a.item.lang}}</span>
            </div>
          </div>
        </a.head>
        <a.body>
          <h1 class="title is-4">{{a.item.name}}</h1>
          <p>{{a.item.desc}}</p>
          <p><a href="{{a.item.link}}">Learn more...</a></p>
        </a.body>
      </ListAccordion>
      `,
    context: {
      products: productMetadata,
    },
  };
};

export let NotExpandable = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Accordion, not expandable</h5>
      <ListAccordion @source={{products}} @key="name" as |a|>
        <a.head @buttonLabel="details" @isExpandable={{eq a.item.lang "golang"}}>
          <div class="columns inline-definitions">
            <div class="column is-1">{{a.item.name}}</div>
            <div class="column is-1">
              <span class="bumper-left badge is-light">{{a.item.lang}}</span>
            </div>
          </div>
        </a.head>
        <a.body>
          <h1 class="title is-4">{{a.item.name}}</h1>
          <p>{{a.item.desc}}</p>
          <p><a href="{{a.item.link}}">Learn more...</a></p>
        </a.body>
      </ListAccordion>
      `,
    context: {
      products: productMetadata,
    },
  };
};

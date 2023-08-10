/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import productMetadata from '../../app/utils/styleguide/product-metadata';

import EmberObject, { computed } from '@ember/object';

import { getOwner } from '@ember/application';
import { on } from '@ember/object/evented';
import Controller from '@ember/controller';

export default {
  title: 'Components/Table',
};

/**
 * The Ember integration for Storybook renders a container component with no routing,
 * which means things that need query parameters, like sorting and pagination, won’t work.

 * This initialiser turns on routing and accepts a controller definition that gets wired up
 * to a generated `storybook` route. The controller is attached to the Storybook component
 * as the `controller` property so its query parameters are accessible from the template.
 */
function injectRoutedController(controllerClass) {
  return on('init', function () {
    let container = getOwner(this);
    container.register('controller:storybook', controllerClass);

    let routerFactory = container.factoryFor('router:main');
    routerFactory.class.map(function () {
      this.route('storybook');
    });

    /* eslint-disable-next-line ember/no-private-routing-service */
    let router = container.lookup('router:main');
    router.initialURL = 'storybook';
    router.startRouting(true);

    this.set('controller', container.lookup('controller:storybook'));
  });
}

let longList = [
  {
    city: 'New York',
    growth: 0.048,
    population: '8405837',
    rank: '1',
    state: 'New York',
  },
  {
    city: 'Los Angeles',
    growth: 0.048,
    population: '3884307',
    rank: '2',
    state: 'California',
  },
  {
    city: 'Chicago',
    growth: -0.061,
    population: '2718782',
    rank: '3',
    state: 'Illinois',
  },
  {
    city: 'Houston',
    growth: 0.11,
    population: '2195914',
    rank: '4',
    state: 'Texas',
  },
  {
    city: 'Philadelphia',
    growth: 0.026,
    population: '1553165',
    rank: '5',
    state: 'Pennsylvania',
  },
  {
    city: 'Phoenix',
    growth: 0.14,
    population: '1513367',
    rank: '6',
    state: 'Arizona',
  },
  {
    city: 'San Antonio',
    growth: 0.21,
    population: '1409019',
    rank: '7',
    state: 'Texas',
  },
  {
    city: 'San Diego',
    growth: 0.105,
    population: '1355896',
    rank: '8',
    state: 'California',
  },
  {
    city: 'Dallas',
    growth: 0.056,
    population: '1257676',
    rank: '9',
    state: 'Texas',
  },
  {
    city: 'San Jose',
    growth: 0.105,
    population: '998537',
    rank: '10',
    state: 'California',
  },
  {
    city: 'Austin',
    growth: 0.317,
    population: '885400',
    rank: '11',
    state: 'Texas',
  },
  {
    city: 'Indianapolis',
    growth: 0.078,
    population: '843393',
    rank: '12',
    state: 'Indiana',
  },
  {
    city: 'Jacksonville',
    growth: 0.143,
    population: '842583',
    rank: '13',
    state: 'Florida',
  },
  {
    city: 'San Francisco',
    growth: 0.077,
    population: '837442',
    rank: '14',
    state: 'California',
  },
  {
    city: 'Columbus',
    growth: 0.148,
    population: '822553',
    rank: '15',
    state: 'Ohio',
  },
  {
    city: 'Charlotte',
    growth: 0.391,
    population: '792862',
    rank: '16',
    state: 'North Carolina',
  },
  {
    city: 'Fort Worth',
    growth: 0.451,
    population: '792727',
    rank: '17',
    state: 'Texas',
  },
  {
    city: 'Detroit',
    growth: -0.271,
    population: '688701',
    rank: '18',
    state: 'Michigan',
  },
  {
    city: 'El Paso',
    growth: 0.194,
    population: '674433',
    rank: '19',
    state: 'Texas',
  },
  {
    city: 'Memphis',
    growth: -0.053,
    population: '653450',
    rank: '20',
    state: 'Tennessee',
  },
  {
    city: 'Seattle',
    growth: 0.156,
    population: '652405',
    rank: '21',
    state: 'Washington',
  },
  {
    city: 'Denver',
    growth: 0.167,
    population: '649495',
    rank: '22',
    state: 'Colorado',
  },
  {
    city: 'Washington',
    growth: 0.13,
    population: '646449',
    rank: '23',
    state: 'District of Columbia',
  },
  {
    city: 'Boston',
    growth: 0.094,
    population: '645966',
    rank: '24',
    state: 'Massachusetts',
  },
  {
    city: 'Nashville-Davidson',
    growth: 0.162,
    population: '634464',
    rank: '25',
    state: 'Tennessee',
  },
  {
    city: 'Baltimore',
    growth: -0.04,
    population: '622104',
    rank: '26',
    state: 'Maryland',
  },
  {
    city: 'Oklahoma City',
    growth: 0.202,
    population: '610613',
    rank: '27',
    state: 'Oklahoma',
  },
  {
    city: 'Louisville/Jefferson County',
    growth: 0.1,
    population: '609893',
    rank: '28',
    state: 'Kentucky',
  },
  {
    city: 'Portland',
    growth: 0.15,
    population: '609456',
    rank: '29',
    state: 'Oregon',
  },
  {
    city: 'Las Vegas',
    growth: 0.245,
    population: '603488',
    rank: '30',
    state: 'Nevada',
  },
  {
    city: 'Milwaukee',
    growth: 0.003,
    population: '599164',
    rank: '31',
    state: 'Wisconsin',
  },
  {
    city: 'Albuquerque',
    growth: 0.235,
    population: '556495',
    rank: '32',
    state: 'New Mexico',
  },
  {
    city: 'Tucson',
    growth: 0.075,
    population: '526116',
    rank: '33',
    state: 'Arizona',
  },
  {
    city: 'Fresno',
    growth: 0.183,
    population: '509924',
    rank: '34',
    state: 'California',
  },
  {
    city: 'Sacramento',
    growth: 0.172,
    population: '479686',
    rank: '35',
    state: 'California',
  },
  {
    city: 'Long Beach',
    growth: 0.015,
    population: '469428',
    rank: '36',
    state: 'California',
  },
  {
    city: 'Kansas City',
    growth: 0.055,
    population: '467007',
    rank: '37',
    state: 'Missouri',
  },
  {
    city: 'Mesa',
    growth: 0.135,
    population: '457587',
    rank: '38',
    state: 'Arizona',
  },
  {
    city: 'Virginia Beach',
    growth: 0.051,
    population: '448479',
    rank: '39',
    state: 'Virginia',
  },
  {
    city: 'Atlanta',
    growth: 0.062,
    population: '447841',
    rank: '40',
    state: 'Georgia',
  },
  {
    city: 'Colorado Springs',
    growth: 0.214,
    population: '439886',
    rank: '41',
    state: 'Colorado',
  },
  {
    city: 'Omaha',
    growth: 0.059,
    population: '434353',
    rank: '42',
    state: 'Nebraska',
  },
  {
    city: 'Raleigh',
    growth: 0.487,
    population: '431746',
    rank: '43',
    state: 'North Carolina',
  },
  {
    city: 'Miami',
    growth: 0.149,
    population: '417650',
    rank: '44',
    state: 'Florida',
  },
  {
    city: 'Oakland',
    growth: 0.013,
    population: '406253',
    rank: '45',
    state: 'California',
  },
  {
    city: 'Minneapolis',
    growth: 0.045,
    population: '400070',
    rank: '46',
    state: 'Minnesota',
  },
  {
    city: 'Tulsa',
    growth: 0.013,
    population: '398121',
    rank: '47',
    state: 'Oklahoma',
  },
  {
    city: 'Cleveland',
    growth: -0.181,
    population: '390113',
    rank: '48',
    state: 'Ohio',
  },
  {
    city: 'Wichita',
    growth: 0.097,
    population: '386552',
    rank: '49',
    state: 'Kansas',
  },
  {
    city: 'Arlington',
    growth: 0.133,
    population: '379577',
    rank: '50',
    state: 'Texas',
  },
];

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table</h5>
      <ListTable @source={{shortList}} as |t|>
        <t.head>
          <th>Name</th>
          <th>Language</th>
          <th>Description</th>
        </t.head>
        <t.body @key="model.name" as |row|>
          <tr>
            <td>{{row.model.name}}</td>
            <td>{{row.model.lang}}</td>
            <td>{{row.model.desc}}</td>
          </tr>
        </t.body>
      </ListTable>
      <p class="annotation">Tables have airy designs with a minimal amount of borders. This maximizes their utility.</p>
      `,
    context: {
      shortList: productMetadata,
    },
  };
};

export let Search = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table search</h5>
      <div class="boxed-section">
        <div class="boxed-section-head">
          Table Name
          <SearchBox
            @searchTerm={{mut controller.searchTerm}}
            @placeholder="Search..."
            @class="is-inline pull-right"
            @inputClass="is-compact" />
        </div>
        <div class="boxed-section-body {{if controller.filteredShortList.length "is-full-bleed"}}">
          {{#if controller.filteredShortList.length}}
            <ListTable @source={{controller.filteredShortList}} as |t|>
              <t.head>
                <th>Name</th>
                <th>Language</th>
                <th>Description</th>
              </t.head>
              <t.body @key="model.name" as |row|>
                <tr>
                  <td>{{row.model.name}}</td>
                  <td>{{row.model.lang}}</td>
                  <td>{{row.model.desc}}</td>
                </tr>
              </t.body>
            </ListTable>
          {{else}}
            <div class="empty-message">
              <h3 class="empty-message-headline">No Matches</h3>
              <p class="empty-message-body">No products match your query.</p>
            </div>
          {{/if}}
        </div>
      </div>
      <p class="annotation">Tables compose with boxed-section and boxed-section composes with search box.</p>
      `,
    context: {
      controller: EmberObject.extend({
        searchTerm: '',

        filteredShortList: computed('searchTerm', function () {
          let term = this.searchTerm.toLowerCase();
          return productMetadata.filter((product) =>
            product.name.toLowerCase().includes(term)
          );
        }),
      }).create(),
    },
  };
};

export let SortableColumns = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table with sortable columns</h5>
      <ListTable @source={{sortedShortList}} @sortProperty={{controller.sortProperty}} @sortDescending={{controller.sortDescending}} as |t|>
        <t.head>
          <t.sort-by @prop="name">Name</t.sort-by>
          <t.sort-by @prop="lang" @class="is-2">Language</t.sort-by>
          <th>Description</th>
        </t.head>
        <t.body @key="model.name" as |row|>
          <tr>
            <td>{{row.model.name}}</td>
            <td>{{row.model.lang}}</td>
            <td>{{row.model.desc}}</td>
          </tr>
        </t.body>
      </ListTable>
      <p class="annotation">The list-table component provides a <code>sort-by</code> contextual component for building <code>link-to</code> components with the appropriate query params.</p>
      <p class="annotation">This leaves the component stateless, relying on data to be passed down and sending actions back up via the router (via link-to).</p>
      `,
    context: {
      injectRoutedController: injectRoutedController(
        Controller.extend({
          queryParams: ['sortProperty', 'sortDescending'],
          sortProperty: 'name',
          sortDescending: false,
        })
      ),

      sortedShortList: computed(
        'controller.{sortProperty,sortDescending}',
        function () {
          let sorted = productMetadata.sortBy(
            this.get('controller.sortProperty') || 'name'
          );
          return this.get('controller.sortDescending')
            ? sorted.reverse()
            : sorted;
        }
      ),
    },
  };
};

export let MultiRow = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Multi-row Table</h5>
      <ListTable @source={{sortedShortList}} @sortProperty={{controller.sortProperty}} @sortDescending={{controller.sortDescending}} @class="is-striped" as |t|>
        <t.head>
          <t.sort-by @prop="name">Name</t.sort-by>
          <t.sort-by @prop="lang">Language</t.sort-by>
        </t.head>
        <t.body @key="model.name" as |row|>
          <tr>
            <td>{{row.model.name}}</td>
            <td>{{row.model.lang}}</td>
          </tr>
          <tr>
            <td colspan="2">{{row.model.desc}}</td>
          </tr>
        </t.body>
      </ListTable>
      <p class="annotation">The list-table component attempts to be as flexible as possible. For this reason, <code>t.body</code> does not provide the typical <code>tr</code> element. It's sometimes desired to have multiple elements per record.</p>
      `,
    context: {
      injectRoutedController: injectRoutedController(
        Controller.extend({
          queryParams: ['sortProperty', 'sortDescending'],
          sortProperty: 'name',
          sortDescending: false,
        })
      ),

      sortedShortList: computed(
        'controller.{sortProperty,sortDescending}',
        function () {
          let sorted = productMetadata.sortBy(
            this.get('controller.sortProperty') || 'name'
          );
          return this.get('controller.sortDescending')
            ? sorted.reverse()
            : sorted;
        }
      ),
    },
  };
};

export let Pagination = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table pagination</h5>
      <ListPagination @source={{longList}} @size={{5}} @page={{controller.currentPage}} as |p|>
        <ListTable @source={{p.list}} @class="with-foot" as |t|>
          <t.head>
            <th class="is-1">Rank</th>
            <th>City</th>
            <th>State</th>
            <th>Population</th>
            <th>Growth</th>
          </t.head>
          <t.body @key="model.rank" as |row|>
            <tr>
              <td>{{row.model.rank}}</td>
              <td>{{row.model.city}}</td>
              <td>{{row.model.state}}</td>
              <td>{{row.model.population}}</td>
              <td>{{format-percentage row.model.growth total=1}}</td>
            </tr>
          </t.body>
        </ListTable>
        <div class="table-foot">
          <nav class="pagination">
            <span class="bumper-left">U.S. City population and growth from 2000-2013</span>
            <div class="pagination-numbers">
              {{p.startsAt}}&ndash;{{p.endsAt}} of {{longList.length}}
            </div>
              <p.prev @class="pagination-previous"> &lt; </p.prev>
              <p.next @class="pagination-next"> &gt; </p.next>
              <ul class="pagination-list"></ul>
          </nav>
        </div>
      </ListPagination>
      <p class="annotation">Pagination works like sorting: using <code>link-to</code>s to set a query param.</p>
      <p class="annotation">Pagination, like Table, is a minimal design. Only a next and previous button are available. The current place in the set of pages is tracked by showing which slice of items is currently shown.</p>
      <p class="annotation">The pagination component exposes first and last components (for jumping to the beginning and end of a list) as well as pageLinks for generating links around the current page.</p>
      `,
    context: {
      injectRoutedController: injectRoutedController(
        Controller.extend({
          queryParams: ['currentPage'],
          currentPage: 1,
        })
      ),
      longList,
    },
  };
};

export let RowLinks = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table row links</h5>
      <ListTable @source={{shortList}} as |t|>
        <t.head>
          <th>Name</th>
          <th>Language</th>
          <th>Description</th>
        </t.head>
        <t.body @key="model.name" as |row|>
          <tr class="is-interactive">
            <td><a href="javascript:;" class="is-primary">{{row.model.name}}</a></td>
            <td>{{row.model.lang}}</td>
            <td>{{row.model.desc}}</td>
          </tr>
        </t.body>
      </ListTable>
      <p class="annotation">It is common for tables to act as lists of links, (e.g., clients list all allocations, each row links to the allocation detail). The helper class <code>is-interactive</code> on the <code>tr</code> makes table rows have a pointer cursor. The helper class <code>is-primary</code> on the <code>a</code> element in a table row makes the link bold and black instead of blue. This makes the link stand out less, since the entire row is a link.</p>
      <p class="annotation">
        A few rules for using table row links:
        <ol>
          <li>The <code>is-primary</code> cell should always be the first cell</li>
          <li>The <code>is-primary</code> cell should always contain a link to the destination in the form of an <code>a</code> element. This is to support opening a link in a new tab.</li>
          <li>The full row should transition to the destination on click. This is to improve the usability of a table by creating a larger click area.</li>
        </ol>
      </p>
      `,
    context: {
      shortList: productMetadata,
    },
  };
};

export let CellLinks = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table cell links</h5>
      <ListTable @source={{shortList}} as |t|>
        <t.head>
          <th>Name</th>
          <th>Language</th>
          <th>Description</th>
        </t.head>
        <t.body @key="model.name" as |row|>
          <tr>
            <td><a href={{row.model.link}} target="_parent">{{row.model.name}}</a></td>
            <td>{{row.model.lang}}</td>
            <td>{{row.model.desc}}</td>
          </tr>
        </t.body>
      </ListTable>
      <p class="annotation">Links in table cells are just links.</p>
      `,
    context: {
      shortList: productMetadata,
    },
  };
};

export let CellDecorations = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table cell decorations</h5>
      <ListTable @source={{shortList}} as |t|>
        <t.head>
          <th>Name</th>
          <th>Language</th>
          <th>Description</th>
        </t.head>
        <t.body @key="model.name" as |row|>
          <tr>
            <td><a href={{row.model.link}}>{{row.model.name}}</a></td>
            <td class="nowrap">
              <span class="color-swatch
                {{if (eq row.model.lang "ruby") "swatch-6"}}
                {{if (eq row.model.lang "golang") "swatch-5"}}" />
              {{row.model.lang}}
            </td>
            <td>{{row.model.desc}}</td>
          </tr>
        </t.body>
      </ListTable>
      <p class="annotation">Small icons and accents of color make tables easier to scan.</p>
      `,
    context: {
      shortList: productMetadata,
    },
  };
};

export let CellIcons = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Table cell icons</h5>
      <ListPagination @source={{longList}} @size={{5}} @page={{controller.currentPage}} as |p|>
        <ListTable @source={{p.list}} @class="with-foot" as |t|>
          <t.head>
            <th class="is-narrow"></th>
            <th class="is-1">Rank</th>
            <th>City</th>
            <th>State</th>
            <th>Population</th>
            <th>Growth</th>
          </t.head>
          <t.body @key="model.rank" as |row|>
            <tr>
              <td class="is-narrow">
                {{#if (lt row.model.growth 0)}}
                  {{x-icon "alert-triangle" class="is-warning"}}
                {{/if}}
              </td>
              <td>{{row.model.rank}}</td>
              <td>{{row.model.city}}</td>
              <td>{{row.model.state}}</td>
              <td>{{row.model.population}}</td>
              <td>{{format-percentage row.model.growth total=1}}</td>
            </tr>
          </t.body>
        </ListTable>
        <div class="table-foot">
          <nav class="pagination">
            <span class="bumper-left">U.S. City population and growth from 2000-2013. Cities with negative growth denoted.</span>
            <div class="pagination-numbers">
              {{p.startsAt}}&ndash;{{p.endsAt}} of {{longList.length}}
            </div>
              <p.prev @class="pagination-previous"> &lt; </p.prev>
              <p.next @class="pagination-next"> &gt; </p.next>
              <ul class="pagination-list"></ul>
          </nav>
        </div>
      </ListPagination>
      `,
    context: {
      injectRoutedController: injectRoutedController(
        Controller.extend({
          queryParams: ['currentPage'],
          currentPage: 1,
        })
      ),
      longList,
    },
  };
};

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'ember-cli-mirage';
import { dasherize } from '@ember/string';
import faker from 'nomad-ui/mirage/faker';
import { pickOne } from '../utils';

const REF_TIME = new Date();
const TROUBLESOME_CHARACTERS = '🏆 💃 🤩 🙌🏿 🖨 ? ; %'.split(' ');
const makeWord = () => (faker.random.number(10000000) + 50000).toString(36);
const makeSentence = (count = 10) =>
  new Array(count).fill(null).map(makeWord).join(' ');

const fileTypeMapping = {
  svg: 'image/svg',
  txt: 'text/plain',
  json: 'application/json',
  app: 'application/octet-stream',
  exe: 'application/octet-stream',
};

const fileBodyMapping = {
  svg: () => `
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 207 60">
  <g fill="none">
    <path class="top" fill="#25BA81" d="M26.03.01L0 15.05l17.56 10.32 3.56-2.17 8.63 4.82v-10l8.27-4.97v10.02l14.48-8.02v-.04"/>
    <path class="left" fill="#25BA81" d="M22.75 32.03v9.99l-7.88 5v-20l2.99-1.83L.15 15.05H0v29.96l26.25 15V34.03"/>
    <path class="right" fill="#1F9967" d="M38.02 23.07v9.95l-6.93 4.01-4.84-3v25.98h.14l26.11-15V15.05l-.49-.01"/>
    <path class="text" fill="#000" d="M78.49 21.83v24.24h-5.9v-32h8.06l12.14 24.32V14.1h5.9v32h-8.06m22.46.45c-8 0-10.18-4.42-10.18-9.22v-5.9c0-4.8 2.16-9.22 10.18-9.22s10.18 4.42 10.18 9.22v5.91c0 4.79-2.16 9.21-10.18 9.21zm0-19.35c-3.12 0-4.32 1.39-4.32 4v6.29c0 2.64 1.2 4 4.32 4s4.32-1.39 4.32-4v-6.25c0-2.64-1.2-4.04-4.32-4.04zm27.99 18.87V29.75c0-1.25-.53-1.87-1.87-1.87-2.147.252-4.22.932-6.1 2v16.19h-5.86V22.69h4.46l.58 2c2.916-1.46 6.104-2.293 9.36-2.45 1.852-.175 3.616.823 4.42 2.5 2.922-1.495 6.13-2.348 9.41-2.5 3.89 0 5.28 2.74 5.28 6.91v16.92h-5.86V29.75c0-1.25-.53-1.87-1.87-1.87-2.15.234-4.23.915-6.1 2v16.19h-5.85zm41.81 0h-4.8l-.43-1.58c-2.084 1.352-4.516 2.068-7 2.06-4.27 0-6.1-2.93-6.1-7 0-4.75 2.06-6.58 6.82-6.58H177v-2.41c0-2.59-.72-3.5-4.46-3.5-2.18.024-4.35.265-6.48.72l-.72-4.46c2.606-.72 5.296-1.09 8-1.1 7.34 0 9.5 2.59 9.5 8.45l.05 15.4zM177 37.24h-4.32c-1.92 0-2.45.53-2.45 2.3 0 1.77.53 2.35 2.35 2.35 1.55-.02 3.07-.434 4.42-1.2v-3.45zm9.48-6.77c0-5.18 2.3-8.26 7.73-8.26 2.097.02 4.187.244 6.24.67v-9.74l5.86-.82v33.75h-4.66l-.58-2c-2.133 1.595-4.726 2.454-7.39 2.45-4.7 0-7.2-2.79-7.2-8.11v-7.94zm14-2.64c-1.702-.38-3.437-.588-5.18-.62-2.11 0-2.93 1-2.93 3.12v8.26c0 1.92.72 3 2.88 3 1.937-.07 3.787-.816 5.23-2.11V27.83z"/>
  </g>
</svg>
  `,
  txt: () =>
    new Array(3000)
      .fill(null)
      .map((_, i) => {
        const date = new Date(2019, 6, 23);
        date.setSeconds(i * 5);
        return `${date.toISOString()} ${makeSentence(
          faker.random.number({ max: 5 }) + 7
        )}`;
      })
      .join('\n'),
  json: () =>
    JSON.stringify({
      key: 'value',
      array: [1, 'two', [3]],
      deep: {
        ly: {
          nest: 'ed',
        },
      },
    }),
};

export default Factory.extend({
  id: (i) => i,

  isDir: faker.random.boolean,

  // Depth is used to recursively create nested directories.
  depth: 0,
  parent: null,

  fileType() {
    if (this.isDir) return 'dir';
    return pickOne(['svg', 'txt', 'json', 'app', 'exe']);
  },

  contentType() {
    return fileTypeMapping[this.fileType] || null;
  },

  path() {
    if (this.parent) {
      return `${this.parent.path}/${this.name}`;
    }

    return this.name;
  },

  name() {
    return `${dasherize(faker.hacker.noun())}-${pickOne(
      TROUBLESOME_CHARACTERS
    )}${this.isDir ? '' : `.${this.fileType}`}`;
  },

  body() {
    const strategy = fileBodyMapping[this.fileType];
    return strategy ? strategy() : '';
  },

  size() {
    return this.body.length;
  },

  modTime: () => faker.date.past(2 / 365, REF_TIME),

  dir: trait({
    isDir: true,
    afterCreate(allocFile, server) {
      // create files for the directory
      if (allocFile.depth > 0) {
        server.create('allocFile', 'dir', {
          parent: allocFile,
          depth: allocFile.depth - 1,
        });
      }

      server.createList(
        'allocFile',
        faker.random.number({ min: 1, max: 3 }),
        'file',
        {
          parent: allocFile,
        }
      );
    },
  }),

  file: trait({
    isDir: false,
  }),
});

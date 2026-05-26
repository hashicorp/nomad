/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import FsBreadcrumbs from 'nomad-ui/components/fs/breadcrumbs';
import FsDirectoryEntry from 'nomad-ui/components/fs/directory-entry';
import FsFile from 'nomad-ui/components/fs/file';
import ListTable from 'nomad-ui/components/list-table';

export default class Browser extends Component {
  get allocation() {
    return this.args.model?.allocation || this.args.model;
  }

  get taskState() {
    if (this.args.model?.allocation) {
      return this.args.model;
    }

    return undefined;
  }

  get directoryEntriesArray() {
    return (
      this.args.directoryEntries?.toArray?.() ||
      this.args.directoryEntries ||
      []
    );
  }

  get directories() {
    return this.directoryEntriesArray.filter((entry) => entry.IsDir);
  }

  get files() {
    return this.directoryEntriesArray.filter((entry) => !entry.IsDir);
  }

  get sortedDirectoryEntries() {
    const sortProperty = this.args.sortProperty;
    const directorySortProperty =
      sortProperty === 'Size' ? 'Name' : sortProperty;

    const sortedDirectories = this.directories
      .slice()
      .sort((left, right) =>
        compareEntries(left, right, directorySortProperty),
      );
    const sortedFiles = this.files
      .slice()
      .sort((left, right) => compareEntries(left, right, sortProperty));

    const sortedDirectoryEntries = sortedDirectories.concat(sortedFiles);

    if (this.args.sortDescending) {
      return sortedDirectoryEntries.reverse();
    }

    return sortedDirectoryEntries;
  }

  <template>
    <section class="section is-closer {{if @isFile 'is-full-width'}}">
      {{#if @isFile}}
        <FsFile
          @allocation={{this.allocation}}
          @taskState={{this.taskState}}
          @file={{@path}}
          @stat={{@stat}}
          class="fs-explorer"
        >
          <FsBreadcrumbs
            @allocation={{this.allocation}}
            @taskState={{this.taskState}}
            @path={{@path}}
          />
        </FsFile>
      {{else}}
        <div class="fs-explorer boxed-section">
          <div class="boxed-section-head">
            <FsBreadcrumbs
              @allocation={{this.allocation}}
              @taskState={{this.taskState}}
              @path={{@path}}
            />
          </div>
          {{#if @directoryEntries}}
            <ListTable
              @source={{this.sortedDirectoryEntries}}
              @sortProperty={{@sortProperty}}
              @sortDescending={{@sortDescending}}
              @class="boxed-section-body is-full-bleed is-compact"
              as |t|
            >
              <t.head>
                <t.sortBy @prop="Name" @class="is-two-thirds">Name</t.sortBy>
                <t.sortBy @prop="Size" @class="has-text-right">File Size</t.sortBy>
                <t.sortBy @prop="ModTime" @class="has-text-right">Last Modified</t.sortBy>
              </t.head>
              <t.body as |row|>
                <FsDirectoryEntry
                  @path={{@path}}
                  @allocation={{this.allocation}}
                  @taskState={{this.taskState}}
                  @entry={{row.model}}
                />
              </t.body>
            </ListTable>
          {{else}}
            <div class="boxed-section-body">
              <div data-test-empty-directory class="empty-message">
                <h3
                  data-test-empty-directory-headline
                  class="empty-message-headline"
                >No Files</h3>
                <p data-test-empty-directory-body class="empty-message-body">
                  Directory is currently empty.
                </p>
              </div>
            </div>
          {{/if}}
        </div>
      {{/if}}
    </section>
  </template>
}

function compareEntries(left, right, sortProperty) {
  const leftValue = left?.[sortProperty];
  const rightValue = right?.[sortProperty];

  if (typeof leftValue === 'string' && typeof rightValue === 'string') {
    return leftValue.localeCompare(rightValue);
  }

  if (leftValue === rightValue) {
    return 0;
  }

  return leftValue > rightValue ? 1 : -1;
}

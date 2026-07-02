/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import AppBreadcrumbs from 'nomad-ui/components/app-breadcrumbs';
import GlobalHeader from 'nomad-ui/components/global-header';
import GutterMenu from 'nomad-ui/components/gutter-menu';

export default class PageLayout extends Component {
  @tracked isGutterOpen = false;

  openGutter = () => {
    this.isGutterOpen = true;
  };

  closeGutter = () => {
    this.isGutterOpen = false;
  };

  <template>
    <div class="page-layout" ...attributes>
      <GlobalHeader class="page-header" @onHamburgerClick={{this.openGutter}}>
        <AppBreadcrumbs />
      </GlobalHeader>
      <GutterMenu
        class="page-body"
        @isOpen={{this.isGutterOpen}}
        @onHamburgerClick={{this.closeGutter}}
      >
        {{yield}}
      </GutterMenu>
    </div>
  </template>
}

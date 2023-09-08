/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Components/Header',
};

export let Header = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Global header</h5>
      <nav class="navbar is-primary">
        <div class="navbar-brand">
          <span class="gutter-toggle" aria-label="menu">
            <HamburgerMenu />
          </span>
          <span class="navbar-item is-logo">
            <NomadLogo />
          </span>
        </div>
        <div class="navbar-end">
          <a class="navbar-item">Secondary</a>
          <a class="navbar-item">Links</a>
          <a class="navbar-item">Here</a>
        </div>
      </nav>
      `,
  };
};

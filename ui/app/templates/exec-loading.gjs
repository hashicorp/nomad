/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import LoadingSpinner from 'nomad-ui/components/loading-spinner';
import NomadLogo from 'nomad-ui/components/nomad-logo';

<template>
  {{pageTitle "Exec"}}
  <nav class="navbar is-popup">
    <div class="navbar-brand">
      <div class="navbar-item is-logo">
        <NomadLogo />
      </div>
    </div>

    <div class="navbar-end">
      <a
        href="https://developer.hashicorp.com/nomad/docs"
        target="_blank"
        rel="noopener noreferrer"
        class="navbar-item"
      >Documentation</a>
      <HdsIcon @name="lock" />
    </div>
  </nav>

  <div class="exec-window loading">
    <LoadingSpinner />
  </div>
</template>

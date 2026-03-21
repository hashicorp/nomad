/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { pageTitle } from 'ember-page-title';
import AllocationSubnav from 'nomad-ui/components/allocation-subnav';
import FsBrowser from 'nomad-ui/components/fs/browser';

<template>
  {{pageTitle
    @controller.pathWithLeadingSlash
    " - Allocation "
    @controller.allocation.shortId
    " filesystem"
  }}
  <AllocationSubnav @allocation={{@controller.allocation}} />
  <FsBrowser
    @model={{@controller.allocation}}
    @path={{@controller.path}}
    @stat={{@controller.stat}}
    @isFile={{@controller.isFile}}
    @directoryEntries={{@controller.directoryEntries}}
    @sortProperty={{@controller.sortProperty}}
    @sortDescending={{@controller.sortDescending}}
  />
</template>

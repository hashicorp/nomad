/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { pageTitle } from 'ember-page-title';
import {
  HdsAlert,
  HdsFormToggleGroup,
  HdsSeparator,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "User Settings"}}
  <section class="section">
    <HdsAlert @type="inline" @title="Local Storage Settings" as |A|>
      <A.Title>User Settings</A.Title>
      <A.Description>
        These settings will be saved to your browser settings via Local Storage.
      </A.Description>
      <A.Generic>
        <HdsSeparator />
        <HdsFormToggleGroup as |G|>
          <G.ToggleField
            name="word-wrap"
            @id="word-wrap"
            checked={{@controller.wordWrap}}
            {{on "change" @controller.toggleWordWrap}}
            as |F|
          >
            <F.Label>Word Wrap</F.Label>
            <F.HelperText>
              Wrap lines of text in logs and exec terminals in the UI
            </F.HelperText>
          </G.ToggleField>
          <G.ToggleField
            name="jostle"
            @id="jostle"
            checked={{@controller.liveUpdateJobsIndex}}
            {{on "change" @controller.toggleLiveUpdateJobsIndex}}
            as |F|
          >
            <F.Label>Live Updates to
              <LinkTo @route="jobs.index">Jobs Index</LinkTo></F.Label>
            <F.HelperText>
              When enabled, new or removed jobs will pop into and out of view on
              your jobs page. When disabled, you will be notified that changes
              are pending.
            </F.HelperText>
          </G.ToggleField>
        </HdsFormToggleGroup>
      </A.Generic>
    </HdsAlert>
  </section>
</template>

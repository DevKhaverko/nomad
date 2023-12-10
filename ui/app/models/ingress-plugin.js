/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default class IngressPlugin extends Model {
  @attr('string') plainId;
  @attr('string') provider;
  @attr('string') version;

  @fragmentArray('controller', { defaultValue: () => [] }) controllers;
  @attr('number') controllersHealthy;
  @attr('number') controllersExpected;

  @computed('controllersHealthy', 'controllersExpected')
  get controllersHealthyProportion() {
    return this.controllersHealthy / this.controllersExpected;
  }
}

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';

export default class IngressPluginsPluginController extends Controller {
  get plugin() {
    return this.model;
  }

  get breadcrumbs() {
    const { plainId } = this.plugin;
    return [
      {
        label: 'Plugins',
        args: ['ingress.plugins'],
      },
      {
        label: plainId,
        args: ['ingress.plugins.plugin', plainId],
      },
    ];
  }
}

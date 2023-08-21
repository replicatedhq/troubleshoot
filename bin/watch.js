#!/usr/bin/env node

const gri = require('gaze-run-interrupt');

const binList = [
  // 'bin/analyze',
  // 'bin/preflight',
  'bin/support-bundle',
  // 'bin/collect'
]
const makeList = [
  // 'analyze',
  // 'preflight',
  'support-bundle',
  // 'collect'
]

const makeCommands = [
  // {
  //   command: 'rm',
  //   args: binList,
  // },
  {
    command: 'make',
    args: makeList,
  },
];

commands.push({
  command: "date",
  args: [],
});

commands.push({
  command: "echo",
  args: ["synced"],
});

gri([
  'cmd/**/*.go',
  'pkg/**/*.go',
], makeCommands);

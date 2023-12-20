#!/usr/bin/env node

const gri = require('gaze-run-interrupt');

const commands = [
  // {
  //   command: 'rm',
  //   args: binList,
  // },
  {
    command: 'make',
    args: ['build'],
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
], commands);

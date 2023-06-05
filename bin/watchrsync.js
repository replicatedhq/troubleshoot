#!/usr/bin/env node

const gri = require('gaze-run-interrupt');

if (!process.env.REMOTES) {
  console.log("Usage: `REMOTES='user@h1.1.1.1,user@1.1.1.2' ./watchrsync.js`");
  process.exit(1);
}

process.env.GOOS = 'linux';
process.env.GOARCH = 'amd64';

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

const commands = [
  // {
  //   command: 'rm',
  //   args: binList,
  // },
  {
    command: 'make',
    args: makeList,
  },
];

process.env.REMOTES.split(",").forEach(function (remote) {
  commands.push({
    command: 'rsync',
    args: binList.concat(`${remote}:`),
  });
});

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

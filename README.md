# JudgeGo

JudgeGo is my personal Discord bot for my Discord server. The main functionality it provides is the ability to create new sound clips that are stored locally or in S3 that the bot can then play back to you in your own voice channels. The bot is a WIP therefore some of the functionality is hardcoded to my personal server but it shouldn't be too hard to figure out what is hardcoded if you look at the code.

## Getting Started

If you wanted to run your own judgego you can accomplish exactly that by modifying a few environment variables.

* `S3_PERSISTENCE` - Set to true or false to toggle between s3 audio persistence or local file system
* `AWS_ACCESS_KEY_ID` - Access Key for AWS user with permissions to read/write to your bucket
* `AWS_SECRET_ACCESS_KEY` - Secret Key for AWS user with permissions to read/write to your bucket
* `BUCKET_NAME` - Bucket that judgego will save audio files in
* `DISCORD_BOT_TOKEN` - Bot's API token from Discord

You'll need to take care of getting your bot invited to your discord guild but besides that it should fire up. You will want to run/build `cmd/judgego/main.go` file to get an actual runnable binary. The Dockerfile will have some more info about how I build/run the bot.

## Supported Commands

* `$list` - Will list all available audio files
* `$play <sound_name>` - Will play the sound matching the passed in name
* `$rip <sound_name> <youtube_url> <start_time> <end_time>` - Will create a new sound file for playback. **NOTE: time format is `<minute>m<second>s`. If you want 00:01 to 00:03 of a video the command would be `$rip mail https://www.youtube.com/watch?v=dFuUCpBbbHw 0m1s 0m3s`**

## Available Features

1) Ability to list, create, and play audio files.
2) Local/S3 Persistence functionality

## Pseudo-Available Features

These are features that are hardcoded at the moment to my personal server but are functional none-the-less.

1) Hall of Fame - If a post gets 3 👌 reactions it will be posted into the Hall of Fame channel. Hardcoded to my personal server at this time. Some edge cases are abusable with this feature as written, WIP.
2) Very minor censorship through a regex. Hardcoded at this time.

## Next Steps

1) Allow customization of the video file received from Youtube. In some cases the video was a format that didn't seem to work. Haven't been able to reliably replicate.
2) Overhaul errors. I currently use them to ferry messages (user friendly or otherwise) to the user. Should probably extend error to allow a user friendly message to be added alongside the actual error or revisit how to control what message gets sent to the user.
3) Stop exploiting globals as much, temper with some DI. Though having everything as a defacto global wasn't horrible for a project of this size.
4) Potentially move some of the command specific logic into a command specific file.
5) Some integration/E2E tests would be nice. A lot of functionality with IO needs to be tested. More test coverage in general.
6) A bunch of other tweaks, additions, changes that are too numerous to list here.

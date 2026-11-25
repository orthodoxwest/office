# Release notes

What the App Store and Google Play show under "What's new". A pull request with a notable change
a reader will see adds one file here, `<slug>.txt`, with a sentence or two in plain words.
Most pull requests add none: corrections to texts, fixes and internal work go out under the
standing line "Corrections to the texts and improvements to the app."

```
Platforms: android

The ordo's month strip now scrolls to today.
```

The `Platforms:` line is optional and keeps a note to `android` or `ios` (or both, comma
separated); without it a note goes to both apps. Blank lines part paragraphs; a note must fit
Google Play's 500 characters (`python3 scripts/release_notes.py check`, run by `make test` and
CI). Don't rename a note that has shipped unless the rename keeps most of its words: git's rename
detection is what keeps it from going out again.

Each platform's next release carries every note added since the release everyone has (on
Android, production's newest completed release, not one still staged), newest first by when
master gained it, and then drops them from its later releases; the other platform keeps them until it
ships. A Play release over 500 characters keeps the newest notes and ends "And other
improvements." `python3 scripts/release_notes.py show --platform ios` prints what the next
release would say. Leave shipped notes in place; once both platforms have released them they
can be deleted.

The web has no release notes. The promotion workflows tag what each store has fully out
(`android/0.1.<code>`, `ios/<version>`), so `git log android/0.1.2000..android/0.1.2010` lists
what an Android release brought. See `scripts/release_notes.py`.

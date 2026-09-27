import importlib.util
from pathlib import Path
import sys
import unittest

spec = importlib.util.spec_from_file_location("verify_psalms", Path(__file__).with_name("verify-psalms.py"))
v = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = v
spec.loader.exec_module(v)


class VerifyPsalmsTest(unittest.TestCase):
    def test_source_sections_continuations_and_merge(self):
        source = v.parse_source('<p class="vlitemheading">Psalm 119.9-16</p>'
                                '<p class="vlpsalm">First half : second<br>continued.'
                                '<br><span class="vlversenumber">10</span> Next : verse.</p>')
        self.assertEqual([(x.number, x.text) for x in source[119]], [(9, 'First half : second continued.'), (10, 'Next : verse.')])
        self.assertEqual(len(v.merge_verses(source[119], [v.Verse(10, True, 'Next: verse!')], 'repeat')), 2)
        with self.assertRaises(ValueError):
            v.merge_verses(source[119], [v.Verse(10, True, 'Different: verse.')], 'conflict')

    def test_findings_distinguish_words_punctuation_and_pointing(self):
        for text, expected in [('First * second.', None), ('First * second!', 'punctuation'),
                               ('First * another.', 'wording'), ('First second.', 'pointing'),
                               ('First * second * third', 'pointing')]:
            self.assertEqual(v.compare_verse(text, 'First : second.'), expected)

    def test_split_alignment_and_numbering(self):
        source = {148: [v.Verse(1, True, 'One : first.'), v.Verse(2, True, 'Two : second.'), v.Verse(3, True, 'Three : third.')]}
        self.assertEqual(v.compare_file('Psalm 148b\nTwo * second.\n3. Three * third.', source), [])
        findings = v.compare_file('Psalm 148:2-3\n9. Two * second.\nThree * third.', source)
        self.assertEqual([f[2] for f in findings], ['verse-number', 'verse-number'])
        with self.assertRaises(ValueError):
            v.parse_local('bad header\n')


if __name__ == '__main__':
    unittest.main()

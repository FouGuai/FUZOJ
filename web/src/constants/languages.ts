export type EditorLanguage = {
  id: string;
  label: string;
  monaco: string;
  template: string;
};

export const editorLanguages: EditorLanguage[] = [
  {
    id: "cpp",
    label: "C++17",
    monaco: "cpp",
    template: `#include <bits/stdc++.h>
using namespace std;

int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);

    return 0;
}
`,
  },
];

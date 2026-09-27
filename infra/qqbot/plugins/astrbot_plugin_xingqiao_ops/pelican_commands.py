"""Recognize read-only report requests, never requests to run a detector."""


def parse_report_query(text: str):
    if not isinstance(text, str):
        return None
    text = text.strip()
    for name in ('@小星（24小时在线）', '@小星(24小时在线)',
                 '@小星（24 小时在线）', '@小星(24 小时在线)',
                 '@小星（24 小时主打陪伴）', '@小星(24 小时主打陪伴)', '@小星'):
        if text.startswith(name):
            text = text[len(name):].strip()
            break
    parts = text.removeprefix('/').split()
    if not parts or parts[0] not in ('检测报告', '鹈鹕报告'):
        return None
    if len(parts) == 1:
        return 0
    if len(parts) == 2 and parts[1].isascii() and parts[1].isdigit() and 1 <= len(parts[1]) <= 10 and not parts[1].startswith('0'):
        return int(parts[1])
    return None

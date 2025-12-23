#!/usr/bin/python
import sys, requests


def try_url(c):
    r = requests.head(
        f"https://i.pximg.net/c/{c}/user-profile/img/2017/03/23/23/55/24/12309801_426f94bac51c1892324deb91e7caa4e6_50.png",
        headers={
            "Referer": "https://www.pixiv.net/",
            "User-Agent": None,
        },
    )
    # print(r)
    return r.ok


c = "1200x1200"
try:
    c = sys.argv[1]
except IndexError:
    pass

if "x" not in c:
    c = c + "x" + c

if try_url(c):
    print(c)

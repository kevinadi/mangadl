from bs4 import BeautifulSoup
import gevent
from gevent.queue import Queue
from gevent import monkey
import requests
from time import time
from pprint import pprint
from PIL import Image
from StringIO import StringIO

monkey.patch_socket()

sites = {
    'mangareader': {
        'url': 'http://www.mangareader.net',
        'page_list': lambda p: ['http://www.mangareader.net' + x['value'] for x in p.find_all('option')],
        'img': lambda p: p.find(id='img').get('src')
    }
}

class Gevent_queue:
    def __init__(self, pages, workers=4):
        self.tasks = Queue()
        self.pages = pages  # format is [(pagenum, pageurl)]
        self.workers = workers  # num of workers
        self.out = []  # list of results

    def worker(self,n):
        while not self.tasks.empty():
            start_time = time()
            task = self.tasks.get()

            # get page
            res_raw = requests.get(task[1])
            res = BeautifulSoup(res_raw.text, 'html.parser')
            img = sites['mangareader']['img'](res)

            # get img
            jpg_raw = requests.get(img)
            jpg = Image.open(StringIO(jpg_raw.content))

            # finish task
            self.out.append((task[0], img, jpg.size))
            print 'Worker {0} finished {1} in {2}'.format(n, task, time()-start_time)

    def execute(self):
        for page in self.pages:
            self.tasks.put_nowait(page)
        gevent.joinall([gevent.spawn(self.worker, x) for x in xrange(self.workers)])
        return self.out


if __name__ == '__main__':

    # get page 1 + page 1 img + links to other pages
    print 'Getting page list'
    page_raw = requests.get(sites['mangareader']['url'] + '/naruto/2')
    page = BeautifulSoup(page_raw.text, 'html.parser')
    img1 = sites['mangareader']['img'](page)
    # TODO get page 1 jpg here

    # get list of pages
    pages = sites['mangareader']['page_list'](page)
    pages = [(i+1,p) for i, p in enumerate(pages[1:])]
    # pprint(pages)

    # get multiple page
    start_time = time()
    q = Gevent_queue(pages[:12], workers=4)
    res = q.execute()
    res.sort()
    pprint(res)
    print '>>> Time:', time()-start_time



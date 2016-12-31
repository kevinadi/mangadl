from bs4 import BeautifulSoup
import gevent
from gevent.queue import Queue
from gevent import monkey
import requests
from time import time
from pprint import pprint
from StringIO import StringIO
import zipfile
import argparse

monkey.patch_socket()


sites = {
    'mangareader': {
        'url': 'http://www.mangareader.net',
        'page_list': lambda p: ['http://www.mangareader.net' + x['value'] for x in p.find_all('option')],
        'img': lambda p: p.find(id='img').get('src')
    }
}


class Gevent_queue:
    '''
    Generic queue using gevent
        tasks: list of tasks to be executed
        worker_function: the function that executes a task
    Each worker will execute `worker_function(task)`
    >>> q = Gevent_queue(tasks, worker_function)
    >>> q.execute()
    '''
    def __init__(self, tasks, worker_func, workers=4):
        self.queue = Queue()
        self.tasks = tasks  # list of tasks
        self.workers = workers  # number of workers
        self.out = []  # list of results
        self.worker_func = worker_func

    def worker(self, n):
        while not self.queue.empty():
            start_time = time()
            task = self.queue.get()

            # execute worker function
            worker_out = self.worker_func(task)
            self.out.append(worker_out)

            # print 'Worker {0} finished {1} in {2}'.format(n, task, time()-start_time)

    def execute(self):
        for task in self.tasks:
            self.queue.put_nowait(task)
        gevent.joinall([gevent.spawn(self.worker, x) for x in xrange(self.workers)])
        self.out.sort()
        return self.out


class Download_single:
    '''
    Download single chapter
    >>> d = Download_single(sites['mangareader'], '/naruto/2')
    >>> d.execute()
    '''
    def __init__(self, site, path):
        self.site = site
        self.path = path
        filename = filter(None, path.split('/'))
        self.filename = filename[0] + '-' + filename[1].zfill(3)

    def worker_func(self, task):
        '''
        task is of the form (pagenum, pageurl)
        output is (filename, jpg_bytes)
        '''
        for retry in xrange(3):
            try:
                # get page
                res_raw = requests.get(task[1])
                res = BeautifulSoup(res_raw.text, 'html.parser')
                img = self.site['img'](res)

                # get img
                jpg_raw = requests.get(img)

                # finish task
                fname = self.filename + '-' + str(task[0]).zfill(3)
            except Exception as e:
                print 'Caught', type(e), 'retrying...', retry
                gevent.sleep(1)
            else:
                return (fname, jpg_raw.content)

    def execute(self):
        # get page 1 + page 1 img + links to other pages
        print 'Getting page list for', self.path
        page_raw = requests.get(self.site['url'] + self.path)
        page = BeautifulSoup(page_raw.text, 'html.parser')
        img1 = self.site['img'](page)
        jpg1_raw = requests.get(img1)

        # get list of pages
        pages = self.site['page_list'](page)

        # tasks format is [(pagenum, pageurl)]
        tasks = [(i+1,p) for i, p in enumerate(pages[1:])]

        # get multiple pages
        start_time = time()
        q = Gevent_queue(tasks, worker_func=self.worker_func, workers=4)
        q_out = q.execute()
        q_out.insert(0, (self.filename + '-000', jpg1_raw.content))

        # return results
        print '>>>', self.path, 'time:', time()-start_time
        return q_out


class Download_many:
    '''
    Download many chapters
    >>> d = Download_many(sites['mangareader'], ['/naruto/4', '/naruto/5', '/naruto/6'])
    >>> d.execute()
    '''
    def __init__(self, site, paths):
        self.site = site
        self.paths = paths
        self.out = []
        filename1 = filter(None, paths[0].split('/'))
        filename2 = filter(None, paths[-1].split('/'))
        self.filename = filename1[0] + '-' + filename1[1].zfill(3) + '-' + filename2[1].zfill(3)

    def worker_func(self, path):
        d = Download_single(self.site, path)
        self.out += d.execute()

    def execute(self):
        tasks = self.paths
        q = Gevent_queue(tasks, self.worker_func, workers=4)
        start_time = time()
        q.execute()
        self.out.sort()

        # save into cbz
        with zipfile.ZipFile(self.filename + '.cbz', 'w', zipfile.ZIP_STORED) as cbz:
            for img in self.out:
                cbz.writestr(img[0] + '.jpg', img[1])

        print '>>> Overall time', time()-start_time


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('manga_name', help='Manga name')
    parser.add_argument('from_chapter', help='From chapter', type=int)
    parser.add_argument('to_chapter', help='To chapter', type=int)
    args = parser.parse_args()

    chapters = ['/' + args.manga_name + '/{0}'.format(x) for x in xrange(args.from_chapter, args.to_chapter+1)]
    d = Download_many(sites['mangareader'], chapters)
    d.execute()
